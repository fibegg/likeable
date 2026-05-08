package likeable

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const projectArchiveTTL = 90 * 24 * time.Hour

func (s *Server) archiveProjectSource(ctx context.Context, user *User, project *Project) (*ProjectArchive, error) {
	if user == nil || project == nil {
		return nil, fmt.Errorf("user and project are required")
	}
	archiveID := uuid.NewString()
	archiveDir := filepath.Join(s.store.DataDir(), "archives", user.ID)
	if err := os.MkdirAll(archiveDir, 0o700); err != nil {
		return nil, err
	}
	filename := archiveID + ".zip"
	relativePath := filepath.Join("archives", user.ID, filename)
	fullPath := filepath.Join(s.store.DataDir(), relativePath)
	if err := s.writeProjectArchive(ctx, user, project, fullPath); err != nil {
		_ = os.Remove(fullPath)
		return nil, err
	}
	now := time.Now().UTC()
	archive := &ProjectArchive{
		ID:           archiveID,
		UserID:       user.ID,
		ProjectID:    project.ID,
		ProjectTitle: project.Title,
		StoragePath:  relativePath,
		Status:       "ready",
		ExpiresAt:    now.Add(projectArchiveTTL).Format(time.RFC3339Nano),
		CreatedAt:    now.Format(time.RFC3339Nano),
		UpdatedAt:    now.Format(time.RFC3339Nano),
	}
	archive.DownloadURL = s.archiveDownloadURL(archive.ID)
	if err := s.store.UpsertProjectArchive(ctx, archive); err != nil {
		_ = os.Remove(fullPath)
		return nil, err
	}
	return archive, nil
}

func (s *Server) writeProjectArchive(ctx context.Context, user *User, project *Project, targetPath string) error {
	if strings.TrimSpace(project.RepoURL) == "" {
		return writeFallbackProjectArchive(targetPath, user, project, "project source is not available")
	}
	fibe, err := s.fibeClientForProject(ctx, project, user.Email)
	if err != nil {
		return err
	}
	giteaToken, err := fibe.GiteaToken(ctx)
	if err != nil {
		return err
	}
	temp, err := os.MkdirTemp("", "likeable-archive-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	sourceURL := withBasicAuth(project.RepoURL, giteaToken["username"], giteaToken["token"])
	if err := runGit(ctx, temp, "clone", "--depth", "1", sourceURL, "."); err != nil {
		return err
	}
	return zipDirectory(targetPath, temp)
}

func writeFallbackProjectArchive(targetPath string, user *User, project *Project, reason string) error {
	out, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()
	readme, err := zw.Create("README.txt")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(readme, "Likeable project archive\n\nProject: %s\nProject ID: %s\nUser: %s\nReason: %s\nCreated: %s\n", project.Title, project.ID, user.Email, reason, time.Now().UTC().Format(time.RFC3339))
	return err
}

func zipDirectory(targetPath, sourceDir string) error {
	out, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()
	sourceDir = filepath.Clean(sourceDir)
	return filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == sourceDir {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(os.PathSeparator)) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func (s *Server) archiveDownloadURL(id string) string {
	return strings.TrimRight(s.config.BaseURL, "/") + "/api/profile/archives/" + id + "/download"
}
