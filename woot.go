package wooter

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const VolumesDir string = "volumes"
const DiffsDir string = "diffs"

type HCSWoot struct {
	BaseDir string
}

func (c HCSWoot) Unpack(id, parentID string, allParents []string, tar io.Reader) (size int, err error) {
	dest := filepath.Join(c.BaseDir, VolumesDir, id)
	if err := os.MkdirAll(dest, 0700); err != nil {
		return 0, err
	}

	tmpTarDest, err := ioutil.TempDir("", id)
	if err != nil {
		return 0, err
	}

	tarFile, err := os.Create(filepath.Join(tmpTarDest, "layer.tar"))
	if err != nil {
		return 0, err
	}
	fmt.Println(tarFile.Name())

	_, err = io.Copy(tarFile, tar)
	if err != nil {
		return 0, err
	}

	tmpDest, err := ioutil.TempDir("", "woot-tmp")
	if err != nil {
		return 0, err
	}

	tarCmd := exec.Command("tar", "-x", "-C", tmpDest, "-f", tarFile.Name())
	tarCmd.Stdout = os.Stdout
	tarCmd.Stderr = os.Stderr
	if err := tarCmd.Run(); err != nil {
		return 0, err
	}

	info := hcsshim.DriverInfo{
		HomeDir: dest,
		Flavour: 1,
	}
	if err := hcsshim.CreateLayer(info, id, parentID); err != nil {
		return 0, err
	}

	if err := hcsshim.ImportLayer(info, id, tmpDest, allParents); err != nil {
		return 0, err
	}

	return 0, nil
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

	return specs.Spec{
		Root: &specs.Root{
			Path: dest,
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
