package codeup

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/charmbracelet/bubbletea"
)

func ScanRepositoriesAsync(ctx context.Context, client *CodeupClient, cfg *Config, repos []RepoConfig, msgChan chan tea.Msg) {
	var (
		mu         sync.Mutex
		candidates []Candidate
		errs       []error
	)

	// msgMu 保护 msgChan 写入，避免多 goroutine 并发写入竞争
	var msgMu sync.Mutex
	sendMsg := func(msg tea.Msg) {
		msgMu.Lock()
		defer msgMu.Unlock()
		msgChan <- msg
	}

	scanSem := make(chan struct{}, cfg.GetScanConcurrency())
	var wg sync.WaitGroup

	for _, repo := range repos {
		wg.Add(1)
		scanSem <- struct{}{}
		go func(r RepoConfig) {
			defer wg.Done()
			defer func() { <-scanSem }()

			select {
			case <-ctx.Done():
				mu.Lock()
				errs = append(errs, ctx.Err())
				mu.Unlock()
				return
			default:
			}

			sendMsg(ScanProgressMsg{Message: renderRepoChecking(r.DisplayName())})

			localPrintLine := func(format string, a ...any) {
				sendMsg(ScanProgressMsg{Message: fmt.Sprintf(format, a...)})
			}

			branches, err := listMergedBranches(ctx, client, r, cfg, localPrintLine)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("扫描 %s: %w", r.DisplayName(), err))
				mu.Unlock()
				sendMsg(ScanProgressMsg{Message: renderScanError(r.DisplayName(), err)})
				return
			}

			mu.Lock()
			for _, branch := range branches {
				candidates = append(candidates, Candidate{
					RepoName:   r.DisplayName(),
					RepoID:     r.Identity(),
					BranchName: branch,
				})
			}
			mu.Unlock()
		}(repo)
	}

	wg.Wait()

	var finalErr error
	if len(errs) > 0 {
		finalErr = fmt.Errorf("扫描完成但有 %d 个错误: %w", len(errs), errors.Join(errs...))
	}

	sendMsg(ScanDoneMsg{Candidates: candidates, Error: finalErr})
}

type ScanProgressMsg struct {
	Message string
}

type ScanDoneMsg struct {
	Candidates []Candidate
	Error      error
}

type branchResult struct {
	name   string
	merged bool
	err    error
}

type branchScanContext struct {
	repositoryIdentity string
	targetBranch       string
	targetCommit       string
}

func listMergedBranches(ctx context.Context, client *CodeupClient, repo RepoConfig, cfg *Config, printLine func(string, ...any)) ([]string, error) {
	var mergedBranches []string
	page := int64(1)
	pageSize := int64(100)
	targetBranch := cfg.GetTargetBranch()

	repositoryIdentity := repo.Identity()
	targetCommit, err := fetchTargetCommit(ctx, client, repositoryIdentity, targetBranch)
	if err != nil {
		return nil, fmt.Errorf("获取目标分支 %s: %w", targetBranch, err)
	}
	scanCtx := branchScanContext{
		repositoryIdentity: repositoryIdentity,
		targetBranch:       targetBranch,
		targetCommit:       targetCommit,
	}

	compareSem := make(chan struct{}, cfg.GetCompareConcurrency())

	for {
		select {
		case <-ctx.Done():
			return mergedBranches, ctx.Err()
		default:
		}

		branches, err := client.ListBranches(ctx, repo.Identity(), page, pageSize)
		if err != nil {
			return nil, fmt.Errorf("列出分支: %w", err)
		}

		if len(branches) == 0 {
			break
		}

		resultCh := make(chan branchResult, len(branches))
		var wg sync.WaitGroup

		for _, branch := range branches {
			if isProtectedBranch(branch.Name, branch) {
				continue
			}
			if isExcludedBranch(branch.Name, cfg.ExcludePatterns) {
				continue
			}

			wg.Add(1)
			compareSem <- struct{}{}
			go func(b Branch) {
				defer wg.Done()
				defer func() { <-compareSem }()

				select {
				case <-ctx.Done():
					resultCh <- branchResult{name: b.Name, err: ctx.Err()}
					return
				default:
				}

				merged, err := isBranchMerged(ctx, client, scanCtx, b)
				resultCh <- branchResult{name: b.Name, merged: merged, err: err}
			}(branch)
		}

		// 同步等待所有 goroutine 完成，避免泄漏
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		// 等待所有比较完成或上下文取消
		select {
		case <-done:
			close(resultCh)
		case <-ctx.Done():
			// 等待所有 goroutine 退出后再返回，避免泄漏
			<-done
			close(resultCh)
		}

		for result := range resultCh {
			if result.err != nil {
				printLine("%s", renderScanError(result.name, result.err))
				continue
			}
			if result.merged {
				if len(mergedBranches) == 0 {
					printLine("")
				}
				mergedBranches = append(mergedBranches, result.name)
				printLine("%s", renderMergedBranch(result.name))
			}
		}

		if int64(len(branches)) < pageSize {
			break
		}
		page++
	}

	return mergedBranches, nil
}

func fetchTargetCommit(ctx context.Context, client *CodeupClient, repoIdentity, targetBranch string) (string, error) {
	branch, err := client.GetBranch(ctx, repoIdentity, targetBranch)
	if err != nil {
		return "", err
	}
	if branch.Commit == nil {
		return "", nil
	}
	return branch.Commit.ID, nil
}

var protectedNames = map[string]bool{
	"master":     true,
	"main":       true,
	"develop":    true,
	"test":       true,
	"production": true,
	"release":    true,
}

func isProtectedBranch(name string, branch Branch) bool {
	if protectedNames[name] {
		return true
	}

	return isTruthy(branch.Protected)
}

func isExcludedBranch(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(pattern, name) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, name string) bool {
	if pattern == name {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(name) >= len(prefix) && name[:len(prefix)] == prefix
	}
	return false
}

func isBranchMerged(ctx context.Context, client *CodeupClient, scanCtx branchScanContext, branch Branch) (bool, error) {
	if branch.Commit != nil && branch.Commit.ID == scanCtx.targetCommit {
		return false, nil
	}

	resp, err := client.GetCompareDetail(ctx, scanCtx.repositoryIdentity, scanCtx.targetBranch, branch.Name)
	if err != nil {
		return false, fmt.Errorf("比较分支: %w", err)
	}
	if resp == nil || len(resp.Commits) > 0 {
		return false, nil
	}

	return true, nil
}
