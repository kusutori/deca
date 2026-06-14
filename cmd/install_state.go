package cmd

import (
	"runtime"
	"time"

	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/github"
	"github.com/kusutori/deca/internal/install"
	"github.com/kusutori/deca/internal/ui"
)

func installedPackageFromResult(name string, pkg *config.Package, installer *install.Installer, release *github.ReleaseInfo, result *install.InstallResult) config.InstalledPackage {
	if pkg.Versioned && result.BinaryPath != "" && runtime.GOOS != "windows" {
		versionedPath, symlinkErr := install.CreateVersionedSymlink(installer.BinDir, name, release.TagName, result.BinaryPath)
		if symlinkErr != nil {
			ui.Warning.Printf("Warning: failed to create versioned symlink for %s: %v\n", name, symlinkErr)
		} else {
			result.VersionedBinaryPath = versionedPath
		}
	}

	return config.InstalledPackage{
		Repo:                pkg.Repo,
		Version:             release.TagName,
		AssetName:           result.AssetName,
		InstallType:         result.InstallType,
		InstalledAt:         time.Now(),
		SystemPkgName:       result.SystemPkgName,
		VersionedBinaryPath: result.VersionedBinaryPath,
		InstallRoot:         result.InstallRoot,
		ExposedPath:         result.ExposedPath,
		ProductCode:         result.ProductCode,
		LinkType:            result.LinkType,
	}
}
