package codeup

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

func ScanRepositories(ctx context.Context, client *CodeupClient, cfg *Config) ([]Candidate, error) {
	var (
		mu         sync.Mutex
		candidates []Candidate
		errs       []error
		outMu      sync.Mutex
	)

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				outMu.Lock()
				fmt.Printf("\r\033[KHTTP 请求: %d", client.RequestCount())
				outMu.Unlock()
			}
		}
	}()

	printLine := func(format string, a ...any) {
		outMu.Lock()
		fmt.Printf("\r\033[K")
		fmt.Printf(format, a...)
		outMu.Unlock()
	}

	scanSem := make(chan struct{}, cfg.GetScanConcurrency())
	var wg sync.WaitGroup

	for _, repo := range cfg.Repositories {
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

			printLine("正在检查仓库: %s\n", r.DisplayName())
			branches, err := listMergedBranches(ctx, client, r, cfg, printLine)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("扫描 %s: %w", r.DisplayName(), err))
				mu.Unlock()
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
	close(done)

	outMu.Lock()
	fmt.Printf("\r\033[K")
	outMu.Unlock()

	if len(errs) > 0 {
		return candidates, fmt.Errorf("扫描完成但有 %d 个错误: %w", len(errs), errors.Join(errs...))
	}
	return candidates, nil
}

type branchResult struct {
	name   string
	merged bool
	err    error
}

func listMergedBranches(ctx context.Context, client *CodeupClient, repo RepoConfig, cfg *Config, printLine func(string, ...any)) ([]string, error) {
	var mergedBranches []string
	page := int64(1)
	pageSize := int64(100)
	targetBranch := cfg.GetTargetBranch()

	targetCommit, err := fetchTargetCommit(ctx, client, repo.Identity(), targetBranch)
	if err != nil {
		return nil, fmt.Errorf("获取目标分支 %s: %w", targetBranch, err)
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

				merged, err := isBranchMerged(ctx, client, repo.Identity(), b, targetBranch, targetCommit)
				resultCh <- branchResult{name: b.Name, merged: merged, err: err}
			}(branch)
		}

		go func() {
			wg.Wait()
			close(resultCh)
		}()

		for result := range resultCh {
			if result.err != nil {
				printLine("    检查 %s 时出错: %v\n", result.name, result.err)
				continue
			}
			if result.merged {
				mergedBranches = append(mergedBranches, result.name)
				printLine("    发现已合并分支: %s\n", result.name)
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

func isProtectedBranch(name string, branch Branch) bool {
	protectedNames := map[string]bool{
		"master":     true,
		"main":       true,
		"develop":    true,
		"test":       true,
		"production": true,
		"release":    true,
	}

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

func isBranchMerged(ctx context.Context, client *CodeupClient, repositoryIdentity string, branch Branch, targetBranch, targetCommit string) (bool, error) {
	if branch.Commit != nil && branch.Commit.ID == targetCommit {
		return false, nil
	}

	resp, err := client.GetCompareDetail(ctx, repositoryIdentity, targetBranch, branch.Name)
	if err != nil {
		return false, fmt.Errorf("比较分支: %w", err)
	}
	if resp == nil || len(resp.Commits) > 0 {
		return false, nil
	}

	return true, nil
}

func executeDeletions(opts TUIOptions, toDelete []Candidate) Result {
	result := Result{}
	semaphore := make(chan struct{}, 5)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, candidate := range toDelete {
		wg.Add(1)
		go func(candidate Candidate) {
			defer wg.Done()

			select {
			case <-opts.Ctx.Done():
				mu.Lock()
				result.Failed = append(result.Failed, FailedDeletion{Candidate: candidate, Error: opts.Ctx.Err()})
				mu.Unlock()
				return
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			}

			if opts.DryRun {
				fmt.Printf("[DryRun] 将删除 %s: %s\n", candidate.RepoName, candidate.BranchName)
				mu.Lock()
				result.Success = append(result.Success, candidate)
				mu.Unlock()
				return
			}

			fmt.Printf("正在删除 %s: %s... ", candidate.RepoName, candidate.BranchName)
			err := opts.Client.DeleteBranch(opts.Ctx, candidate.RepoID, candidate.BranchName)

			mu.Lock()
			if err != nil {
				fmt.Printf("失败: %v\n", err)
				result.Failed = append(result.Failed, FailedDeletion{Candidate: candidate, Error: err})
			} else {
				fmt.Println("成功")
				result.Success = append(result.Success, candidate)
			}
			mu.Unlock()
		}(candidate)
	}

	wg.Wait()
	return result
}
