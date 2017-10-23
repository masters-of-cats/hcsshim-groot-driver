package wooter

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/windows2016fs/layer"
	"code.cloudfoundry.org/windows2016fs/writer"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const VolumesDir string = "volumes"
const DiffsDir string = "diffs"

type HCSWoot struct {
	BaseDir string
}

func (c HCSWoot) Unpack(id, parentID string, allParents []string, blob io.Reader) (size int, err error) {
	dest := filepath.Join(c.BaseDir, VolumesDir, id)
	if err := os.MkdirAll(dest, 0700); err != nil {
		return 0, err
	}

	blobFile, err := ioutil.TempFile("", "blob")
	if err != nil {
		return 0, err
	}

	blobSize, err := io.Copy(blobFile, blob)
	if err != nil {
		return 0, err
	}

	lm := layer.NewManager(hcsshim.DriverInfo{HomeDir: dest, Flavour: 1}, &writer.Writer{})
	if err := lm.Extract(blobFile.Name(), id, allParents); err != nil {
		return 0, err
	}

	return int(blobSize), nil
}

func (c HCSWoot) Bundle(id string, parentIds []string) (specs.Spec, error) {
	dest := filepath.Join(c.BaseDir, DiffsDir, id)
	if err := os.MkdirAll(dest, 0700); err != nil {
		return specs.Spec{}, err
	}

	parentPaths := []string{}

	for _, parent := range parentIds {
		parentPaths = append(parentPaths, path.Join(c.BaseDir, VolumesDir, parent))
	}

	info := hcsshim.DriverInfo{
		HomeDir: dest,
		Flavour: 1,
	}

	parent := parentIds[len(parentIds)-1]

	if err := hcsshim.CreateSandboxLayer(info, id, parent, parentPaths); err != nil {
		return specs.Spec{}, err
	}
	if err := hcsshim.ActivateLayer(info, id); err != nil {
		return specs.Spec{}, err
	}
	if err := hcsshim.PrepareLayer(info, id, parentPaths); err != nil {
		return specs.Spec{}, err
	}

	volumePath, err := hcsshim.GetLayerMountPath(info, id)
	if err != nil {
		return specs.Spec{}, err
	}

	return specs.Spec{
		Root: &specs.Root{
			Path: volumePath,
		},
		Windows: &specs.Windows{
			LayerFolders: parentPaths,
		},
	}, nil
}

func (c HCSWoot) Exists(id string) bool {
	dest := filepath.Join(c.BaseDir, VolumesDir, id)
	info := hcsshim.DriverInfo{
		HomeDir: dest,
		Flavour: 1,
	}
	result, err := hcsshim.LayerExists(info, id)
	if err != nil {
		return false
	}
	return result
}
