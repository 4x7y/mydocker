package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

//Create a AUFS filesystem as container root workspace
func NewWorkSpace(volume, imageName, containerName string) {
	CreateReadOnlyLayer(imageName)
	CreateWriteLayer(containerName)
	CreateMountPoint(containerName, imageName)
	if volume != "" {
		volumeURLs := strings.Split(volume, ":")
		length := len(volumeURLs)
		if length == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			MountVolume(volumeURLs, containerName)
			log.Infof("NewWorkSpace volume urls %q", volumeURLs)
		} else {
			log.Infof("Volume parameter input is not correct.")
		}
	}
}

//Decompression tar image
func CreateReadOnlyLayer(imageName string) error {
	unTarFolderUrl := RootUrl + "/" + imageName + "/"
	imageUrl := ImageUrl + "/" + imageName + ".tar"
	exist, err := PathExists(unTarFolderUrl)
	if err != nil {
		log.Errorf("%v", err)
		return err
	}
	if !exist {
		if err := os.MkdirAll(unTarFolderUrl, 0622); err != nil {
			log.Errorf("$ %v", err)
			return err
		} else {
			log.Infof("$ mkdir %s -m 0777", unTarFolderUrl)
		}

		if _, err := exec.Command("tar", "-xvf", imageUrl, "-C", unTarFolderUrl).CombinedOutput(); err != nil {
			log.Errorf("%v", err)
			return err
		} else {
			log.Infof("$ tar -xvf %s -C %s", imageUrl, unTarFolderUrl)
		}
	}
	return nil
}

func CreateWriteLayer(containerName string) {
	writeURL := fmt.Sprintf(WriteLayerUrl, containerName)
	if err := os.MkdirAll(writeURL, 0777); err != nil {
		log.Warnf("$ %v", err)
	} else {
		log.Infof("$ mkdir %s -m 0777", writeURL)
	}
}

// Volumes provide the best and most predictable performance for write-heavy workloads.
// This is because they bypass the storage driver and do not incur any of the potential
// overheads introduced by thin provisioning and copy-on-write. Volumes have other
// benefits, such as allowing you to share data among containers and persisting even
// when no running container is using them.

func MountVolume(volumeURLs []string, containerName string) error {
	parentUrl := volumeURLs[0]
	if err := os.Mkdir(parentUrl, 0777); err != nil {
		log.Warnf("$ %v", err)
	} else {
		log.Infof("$ mkdir %s -m 0777", parentUrl)
	}

	containerUrl := volumeURLs[1]
	mntURL := fmt.Sprintf(MntUrl, containerName)
	containerVolumeURL := mntURL + "/" + containerUrl
	if err := os.Mkdir(containerVolumeURL, 0777); err != nil {
		log.Warnf("%v", err)
	} else {
		log.Infof("$ mkdir %s -m 0777", containerVolumeURL)
	}

	dirs := "dirs=" + parentUrl
	_, err := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", containerVolumeURL).CombinedOutput()
	if err != nil {
		log.Errorf("%v", err)
		return err
	} else {
		log.Infof("$ mount -t aufs -o %s none %s", dirs, containerVolumeURL)
		log.Infof("AUFS: %s[rw] -> %s[aufs]", parentUrl, containerVolumeURL)
	}
	return nil
}

func CreateMountPoint(containerName, imageName string) error {
	mntUrl := fmt.Sprintf(MntUrl, containerName)
	if err := os.MkdirAll(mntUrl, 0777); err != nil {
		log.Warnf("%v", err)
		return err
	} else {
		log.Infof("$ mkdir %s -m 0777", mntUrl)
	}

	tmpWriteLayer := fmt.Sprintf(WriteLayerUrl, containerName)
	tmpImageLocation := RootUrl + "/" + imageName
	mntURL := fmt.Sprintf(MntUrl, containerName)
	dirs := "dirs=" + tmpWriteLayer + ":" + tmpImageLocation
	_, err := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", mntURL).CombinedOutput()
	if err != nil {
		log.Errorf("Run command for creating mount point failed %v", err)
		return err
	} else {
		log.Infof("$ mount -t aufs -o %s none %s", dirs, mntURL)
		log.Infof("AUFS: %s[rw], %s[ro] -> %s[aufs]", tmpWriteLayer, tmpImageLocation, mntURL)
	}

	return nil
}

//Delete the AUFS filesystem while container exit
func DeleteWorkSpace(volume, containerName string) {
	if volume != "" {
		volumeURLs := strings.Split(volume, ":")
		length := len(volumeURLs)
		if length == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			DeleteMountPointWithVolume(volumeURLs, containerName)
		} else {
			DeleteMountPoint(containerName)
		}
	} else {
		DeleteMountPoint(containerName)
	}
	DeleteWriteLayer(containerName)
}

func DeleteMountPoint(containerName string) error {
	mntURL := fmt.Sprintf(MntUrl, containerName)
	if err := syscall.Unmount(mntURL, syscall.MNT_FORCE); err != nil {
		log.Errorf("%v", err)
	} else {
		log.Infof("$ umount %s", mntURL)
	}

	if err := os.RemoveAll(mntURL); err != nil {
		log.Errorf("%v", err)
		return err
	} else {
		log.Infof("$ rm -rf %s", mntURL)
	}

	return nil
}

func DeleteMountPointWithVolume(volumeURLs []string, containerName string) error {
	mntURL := fmt.Sprintf(MntUrl, containerName)
	containerUrl := mntURL + "/" + volumeURLs[1]
	if err := syscall.Unmount(containerUrl, syscall.MNT_DETACH); err != nil {
		log.Errorf("%v", err)
	} else {
		log.Infof("$ umount %s", containerUrl)
	}

	if err := syscall.Unmount(mntURL, syscall.MNT_DETACH); err != nil {
		log.Errorf("%v", err)
	} else {
		log.Infof("$ umount %s", mntURL)
	}

	if err := os.RemoveAll(mntURL); err != nil {
		log.Errorf("%v", err)
	} else {
		log.Infof("$ rm -rf %s", mntURL)
	}

	return nil
}

func DeleteWriteLayer(containerName string) {
	writeURL := fmt.Sprintf(WriteLayerUrl, containerName)
	if err := os.RemoveAll(writeURL); err != nil {
		log.Errorf("%v", err)
	} else {
		log.Infof("$ rm -rf %s", writeURL)
	}
}
