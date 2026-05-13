package main

import (
	"context"
	"fmt"
	"sync"
)

func scanRepositories(ctx context.Context, client *CodeupClient, cfg *Config) ([]Candidate, error) {
	var (
		mu         sync.Mutex
		wg         sync.WaitGroup
		candidates []Candidate
		errs       []error
	)

	for _, repo := range cfg.Repositories {
		wg.Add(1)
		go func(r RepoConfig) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				mu.Lock()
				errs = append(errs, ctx.Err())
				mu.Unlock()
				return
			default:
			}

			fmt.Printf("正在检查仓库: %s\n", r.DisplayName())
			branches, err := listMergedBranches(ctx, client, r)
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

	if len(errs) > 0 {
		return candidates, fmt.Errorf("扫描完成但有 %d 个错误: %v", len(errs), errs[0])
	}
	return candidates, nil
}

func listMergedBranches(ctx context.Context, client *CodeupClient, repo RepoConfig) ([]string, error) {
	var mergedBranches []string
	page := int64(1)
	pageSize := int64(100)

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

		type branchResult struct {
			name   string
			merged bool
			err    error
		}

		resultCh := make(chan branchResult, len(branches))
		var wg sync.WaitGroup

		for _, branch := range branches {
			if isProtectedBranch(branch.Name, branch) {
				continue
			}

			wg.Add(1)
			go func(b Branch) {
				defer wg.Done()

				select {
				case <-ctx.Done():
					resultCh <- branchResult{name: b.Name, err: ctx.Err()}
					return
				default:
				}

				merged, err := isBranchMerged(ctx, client, repo.Identity(), b.Name)
				resultCh <- branchResult{name: b.Name, merged: merged, err: err}
			}(branch)
		}

		go func() {
			wg.Wait()
			close(resultCh)
		}()

		for result := range resultCh {
			if result.err != nil {
				fmt.Printf("    检查 %s 时出错: %v\n", result.name, result.err)
				continue
			}
			if result.merged {
				mergedBranches = append(mergedBranches, result.name)
				fmt.Printf("    发现已合并分支: %s\n", result.name)
			}
		}

		if int64(len(branches)) < pageSize {
			break
		}
		page++
	}

	return mergedBranches, nil
}

func isProtectedBranch(name string, branch Branch) bool {
	protectedNames := map[string]bool{
		"master":     true,
		"main":       true,
		"production": true,
		"release":    true,
	}

	if protectedNames[name] {
		return true
	}

	return isTruthy(branch.Protected)
}

func isBranchMerged(ctx context.Context, client *CodeupClient, repositoryIdentity, branchName string) (bool, error) {
	resp, err := client.GetCompareDetail(ctx, repositoryIdentity, branchName)
	if err != nil {
		return false, fmt.Errorf("比较分支: %w", err)
	}
	if resp == nil {
		return false, nil
	}
	return len(resp.Commits) == 0, nil
}

func executeDeletions(ctx context.Context, client *CodeupClient, toDelete []Candidate, dryRun bool) Result {
	result := Result{}
	semaphore := make(chan struct{}, 5)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, candidate := range toDelete {
		wg.Add(1)
		go func(candidate Candidate) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				mu.Lock()
				result.Failed = append(result.Failed, FailedDeletion{Candidate: candidate, Error: ctx.Err()})
				mu.Unlock()
				return
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			}

			if dryRun {
				fmt.Printf("[DryRun] 将删除 %s: %s\n", candidate.RepoName, candidate.BranchName)
				mu.Lock()
				result.Success = append(result.Success, candidate)
				mu.Unlock()
				return
			}

			fmt.Printf("正在删除 %s: %s... ", candidate.RepoName, candidate.BranchName)
			err := client.DeleteBranch(ctx, candidate.RepoID, candidate.BranchName)

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