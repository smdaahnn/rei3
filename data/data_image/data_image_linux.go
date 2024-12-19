//go:build !windows

package data_image

import "os/exec"

func setCheckConvertPath(filePathOverwrite string) bool {

	convertPath = "convert"
	if filePathOverwrite != "" {
		convertPath = filePathOverwrite
	}

	_, err := exec.LookPath(convertPath)
	return err == nil
}
