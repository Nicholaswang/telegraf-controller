package utils

import (
	"crypto/md5"
	"encoding/base64"
	"github.com/golang/glog"
	"github.com/mitchellh/mapstructure"
	"io/ioutil"
	"net"
	"os"
	//"os/exec"
	//"strings"
)

// MergeMap copy keys from a `data` map to a `resultTo` tagged object
func MergeMap(data map[string]string, resultTo interface{}) error {
	if data != nil {
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			WeaklyTypedInput: true,
			Result:           resultTo,
			TagName:          "json",
		})
		if err != nil {
			glog.Warningf("error configuring decoder: %v", err)
		} else {
			if err = decoder.Decode(data); err != nil {
				glog.Warningf("error decoding config: %v", err)
			}
		}
		return err
	}
	return nil
}

// BackendHash calc a base64 encoding of a partial hash of an endpoint
// to be used as a cookie value of the backend on sticky session conf
func BackendHash(endpoint string) string {
	hash := md5.Sum([]byte(endpoint))
	return base64.StdEncoding.EncodeToString(hash[:8])
}

// SendToSocket send strings to a unix socket specified
func SendToSocket(socket string, command string) error {
	c, err := net.Dial("unix", socket)
	if err != nil {
		glog.Warningf("error sending to unix socket: %v", err)
		return err
	}
	sent, err := c.Write([]byte(command))
	if err != nil || sent != len(command) {
		glog.Warningf("error sending to unix socket %s", socket)
		return err
	}
	readBuffer := make([]byte, 2048)
	rcvd, err := c.Read(readBuffer)
	if rcvd > 2 {
		glog.Infof("telegraf stat socket response: \"%s\"", string(readBuffer[:rcvd-2]))
	}
	return nil
}

// checkValidity runs a configuration validity check on a file
func checkValidity(configFile string) error {
	//TODO
	return nil
}

// RewriteConfigFiles safely replaces configuration files with new contents after validation
func RewriteConfigFiles(data []byte, reloadStrategy, configFile string) error {
	tmpf := "/etc/telegraf/new_cfg.erb"

	err := ioutil.WriteFile(tmpf, data, 644)
	if err != nil {
		glog.Warningln("Error writing rendered template to file")
		return err
	}

	err = checkValidity(tmpf)
	if err != nil {
		return err
	}
	err = os.Rename(tmpf, configFile)
	if err != nil {
		glog.Warningln("Error updating config file")
		return err
	}
	err = os.Chmod(configFile, 0644)
	if err != nil {
		glog.Warningln("Error chmod config file")
		return err
	}

	return nil
}
