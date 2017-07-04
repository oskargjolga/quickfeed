package scm

import (
	"context"

	gitlab "github.com/xanzy/go-gitlab"
)

// GitlabSCM implements the SCM interface.
type GitlabSCM struct {
	client *gitlab.Client
}

// NewGitlabSCMClient returns a new GitLab client implementing the SCM interface.
func NewGitlabSCMClient(token string) *GitlabSCM {
	return &GitlabSCM{
		client: gitlab.NewOAuthClient(nil, token),
	}
}

// ListDirectories implements the SCM interface.
func (s *GitlabSCM) ListDirectories(ctx context.Context) ([]*Directory, error) {
	groups, _, err := s.client.Groups.ListGroups(&gitlab.ListGroupsOptions{}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	var directories []*Directory
	for _, group := range groups {
		directories = append(directories, &Directory{
			ID:   group.ID,
			Name: group.Name,
		})
	}
	return directories, nil
}

// CreateDirectory implements the SCM interface.
func (s *GitlabSCM) CreateDirectory(ctx context.Context, opt *CreateDirectoryOptions) (*Directory, error) {
	group, _, err := s.client.Groups.CreateGroup(&gitlab.CreateGroupOptions{
		Name:            &opt.Name,
		Path:            &opt.Name,
		VisibilityLevel: getVisibilityLevel(false),
	}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	return &Directory{
		ID:   group.ID,
		Name: group.Name,
	}, nil
}

// GetDirectory implements the SCM interface.
func (s *GitlabSCM) GetDirectory(ctx context.Context, id int) (*Directory, error) {
	group, _, err := s.client.Groups.GetGroup(id, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	return &Directory{
		ID:   group.ID,
		Name: group.Name,
	}, nil
}

func getVisibilityLevel(private bool) *gitlab.VisibilityLevelValue {
	if private {
		return gitlab.VisibilityLevel(gitlab.PrivateVisibility)
	}
	return gitlab.VisibilityLevel(gitlab.PublicVisibility)
}