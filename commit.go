package main

import (
	log "github.com/sirupsen/logrus"
	"os/exec"
)

func commitContainer(imageName string) {
	mntURL := "/root/mnt"
	imageTar := "/root/" + imageName + ".tar"
	// fmt.Printf("%s", imageTar)
	log.Infof("$ tar -czf %s -C %s .", imageTar, mntURL)
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntURL, ".").CombinedOutput(); err != nil {
		log.Errorf("Tar folder %s error %v", mntURL, err)
	} else {
		log.Infof("$ tar -czf %s -C %s .", imageTar, mntURL)
		log.Infof("Package image: %s", imageTar)
	}
}
