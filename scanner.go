package main

import (
	"fmt"
	"sync"
)

func scanRepositories(client *CodeupClient, cfg *Config) []Candidate {
	var (
		mu         sync.Mutex
		wg         sync.WaitGroup
		candidates []Candidate
	)

	for _, repo := range cfg.Repositories {
		wg.Add(1)
		go func(r RepoConfig) {
			defer wg.Done()

			fmt.Printf("正在检查仓库: %s\n", r.DisplayName())
			branches, err := listMergedBranches(client, r)
			if err != nil {
				fmt.Printf("  扫描 %s 时出错: %v\n", r.DisplayName(), err)
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
	return candidates
}

func listMergedBranches(client *CodeupClient, repo RepoConfig) ([]string, error) {
	var mergedBranches []string
	page := int64(1)
	pageSize := int64(100)

	for {
		branches, err := client.ListBranches(repo.Identity(), page, pageSize)
		if err != nil {
			return nil, fmt.Errorf("列出分支: %w", err)
		}

		if len(branches) == 0 {
			break
		}

		for _, branch := range branches {
			if isProtectedBranch(branch.Name, branch) {
				continue
			}

			merged, err := isBranchMerged(client, repo.Identity(), branch.Name)
			if err != nil {
				fmt.Printf("    检查 %s 时出错: %v\n", branch.Name, err)
				continue
			}

			if merged {
				mergedBranches = append(mergedBranches, branch.Name)
				fmt.Printf("    发现已合并分支: %s\n", branch.Name)
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

func isBranchMerged(client *CodeupClient, repositoryIdentity, branchName string) (bool, error) {
	resp, err := client.GetCompareDetail(repositoryIdentity, branchName)
	if err != nil {
		return false, fmt.Errorf("比较分支: %w", err)
	}
	if resp == nil {
		return false, nil
	}
	return len(resp.Commits) == 0, nil
}

func executeDeletions(client *CodeupClient, toDelete []Candidate, dryRun bool) Result {
	result := Result{}
	semaphore := make(chan struct{}, 5)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, candidate := range toDelete {
		wg.Add(1)
		go func(candidate Candidate) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if dryRun {
				fmt.Printf("[DryRun] 将删除 %s: %s\n", candidate.RepoName, candidate.BranchName)
				mu.Lock()
				result.Success = append(result.Success, candidate)
				mu.Unlock()
				return
			}

			fmt.Printf("正在删除 %s: %s... ", candidate.RepoName, candidate.BranchName)
			err := client.DeleteBranch(candidate.RepoID, candidate.BranchName)

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
