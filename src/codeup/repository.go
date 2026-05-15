package codeup

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func ResolveRepositories(ctx context.Context, client *CodeupClient, repos []RepoConfig) error {
	for i := range repos {
		repo := &repos[i]
		if repo.ID != "" {
			if repo.Name == "" {
				repo.Name = repo.ID
			}
			continue
		}
		if looksLikeFullPath(repo.Name) {
			continue
		}

		resolved, err := findRepositoryByName(ctx, client, repo.Name)
		if err != nil {
			return err
		}

		repo.ID = repositoryIdentity(resolved)
		if repo.Name == "" {
			repo.Name = repositoryDisplayName(resolved)
		}
	}
	return nil
}

func findRepositoryByName(ctx context.Context, client *CodeupClient, name string) (*Repository, error) {
	repos, err := client.ListRepositories(ctx, name, 1, 100)
	if err != nil {
		return nil, fmt.Errorf("查询仓库 %q: %w", name, err)
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("未找到仓库 %q", name)
	}

	var matches []*Repository
	for i := range repos {
		repo := &repos[i]
		if repositoryMatches(repo, name) {
			matches = append(matches, repo)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("仓库名 %q 匹配到多个仓库，请改用完整路径", name)
	}
	if len(repos) == 1 {
		return &repos[0], nil
	}

	return nil, fmt.Errorf("仓库名 %q 搜索到多个结果但没有精确匹配，请使用完整路径", name)
}

func repositoryMatches(repo *Repository, name string) bool {
	candidates := []string{
		repo.Name,
		repo.Path,
		repo.NameWithNamespace,
		repo.PathWithNamespace,
	}

	for _, candidate := range candidates {
		if candidate == name {
			return true
		}
	}
	return false
}

func repositoryIdentity(repo *Repository) string {
	if repo.PathWithNamespace != "" {
		return repo.PathWithNamespace
	}
	if repo.Path != "" {
		return repo.Path
	}
	return strconv.FormatInt(repo.ID, 10)
}

func repositoryDisplayName(repo *Repository) string {
	if repo.NameWithNamespace != "" {
		return repo.NameWithNamespace
	}
	if repo.Name != "" {
		return repo.Name
	}
	return strconv.FormatInt(repo.ID, 10)
}

func looksLikeFullPath(name string) bool {
	return strings.Contains(name, "/")
}
