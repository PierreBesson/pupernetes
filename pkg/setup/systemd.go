// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2018 Datadog, Inc.

package setup

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/pupernetes/pkg/config"
	"github.com/coreos/go-systemd/dbus"
	unit2 "github.com/coreos/go-systemd/unit"
	"github.com/golang/glog"
)

const (
	UnitPath             = "/run/systemd/system"
	customSystemdSection = "X-e2e"
)

func getUnitOptions(unitABSPath string) ([]*unit2.UnitOption, error) {
	f, err := os.OpenFile(unitABSPath, os.O_RDONLY, 0)
	if err != nil {
		glog.Errorf("Cannot open %v", err)
		return nil, err
	}
	defer f.Close()
	opts, err := unit2.Deserialize(f)
	if err != nil {
		glog.Errorf("Cannot deserialize %s: %v", unitABSPath, err)
		return nil, err
	}
	glog.V(4).Infof("Deserialized %s with %d items", unitABSPath, len(opts))
	return opts, nil
}

func (e *Environment) isXe2eRootPathEqual(opts []*unit2.UnitOption) bool {
	for _, elt := range opts {
		if elt.Section != customSystemdSection {
			continue
		}
		if elt.Name == "RootPath" {
			glog.V(4).Infof("Current RootPath in already created systemd unit is: %s", elt.Value)
			return elt.Value == e.rootABSPath
		}
	}
	glog.Error("Cannot find the associated systemd Section/Name in the given options")
	return false
}

func statExecStart(opts []*unit2.UnitOption) error {
	for _, elt := range opts {
		if elt.Section != "Service" {
			continue
		}
		if elt.Name == "ExecStart" {
			commandLine := strings.Split(elt.Value, " ")
			_, err := os.Stat(commandLine[0])
			return err
		}
	}
	return fmt.Errorf("cannot find ExecStart in systemd options")
}

func (e *Environment) writeSystemdUnit(unitOpt []*unit2.UnitOption, unitName string) error {
	unitABSPath := path.Join(UnitPath, unitName)
	_, err := os.Stat(unitABSPath)
	if err == nil {
		glog.V(2).Infof("Already created systemd unit: %s, used clean options are %s", unitName, e.cleanOptions.String())

		// Validate the content
		opts, err := getUnitOptions(unitABSPath)
		if err != nil {
			return err
		}
		if !e.isXe2eRootPathEqual(opts) {
			glog.Warningf("The already created unit %q doesn't use the given %s directory", unitName, e.rootABSPath)
		}
		err = statExecStart(opts)
		if err != nil {
			glog.Errorf("Current ExecStart in %s unit is incorrect: %v", unitABSPath, err)
			return err
		}
		return nil
	}

	// Write
	glog.V(4).Infof("Creating systemd unit %s ...", unitName)
	c := unit2.Serialize(unitOpt)
	b, err := ioutil.ReadAll(c)
	if err != nil {
		glog.Errorf("Cannot read kubelet systemd unit: %v", err)
		return err
	}
	err = ioutil.WriteFile(unitABSPath, b, 0444)
	if err != nil {
		glog.Errorf("Fail to write systemd unit %s: %v", unitABSPath, err)
		return err
	}
	glog.V(4).Infof("Successfully wrote systemd unit %s", unitABSPath)
	return nil
}

func (e *Environment) createEnd2EndSection() []*unit2.UnitOption {
	return []*unit2.UnitOption{
		{
			Section: customSystemdSection,
			Name:    "RootPath",
			Value:   e.rootABSPath,
		},
		{
			Section: customSystemdSection,
			Name:    "HyperkubeVersion",
			Value:   e.binaryHyperkube.version,
		},
		{
			Section: customSystemdSection,
			Name:    "EtcdVersion",
			Value:   e.binaryEtcd.version,
		},
		{
			Section: customSystemdSection,
			Name:    "VaultVersion",
			Value:   e.binaryVault.version,
		},
		{
			Section: customSystemdSection,
			Name:    "CNIVersion",
			Value:   e.binaryCNI.version,
		},
		{
			Section: customSystemdSection,
			Name:    "Timestamp",
			Value:   strconv.Itoa(int(time.Now().Unix())),
		},
	}
}

func (e *Environment) createKubeletUnit() error {
	networkPluginArgs := ""
	if e.isDockerBridge == false {
		networkPluginArgs = "--network-plugin=cni"
	}
	sdKubelet := []*unit2.UnitOption{
		{
			Section: "Unit",
			Name:    "Description",
			Value:   "Hyperkube kubelet for end to end testing",
		},
		{
			Section: "Unit",
			Name:    "After",
			Value:   "network.target",
		},
		{
			Section: "Service",
			Name:    "ExecStart",
			Value: strings.Join([]string{
				e.binaryHyperkube.binaryABSPath,
				"kubelet",
				"--v=4",
				"--allow-privileged",
				"--fail-swap-on=false",
				"--hairpin-mode=none",
				"--pod-manifest-path=" + e.manifestStaticPodABSPath,
				"--hostname-override=" + e.GetHostname(),
				"--root-dir=" + e.kubeletRootDir,
				"--healthz-port=" + strconv.Itoa(e.GetKubeletHealthzPort()), // TODO conf this
				"--cert-dir=" + path.Join(e.kubeletRootDir, "pki"),
				"--kubeconfig=" + e.GetKubeconfigInsecurePath(),
				`--cloud-provider=""`,

				"--resolv-conf=" + e.GetResolvConfPath(), // TODO this flag is ignored
				"--cluster-dns=" + e.dnsClusterIP.String(),
				"--cluster-domain=cluster.local",

				"--cert-dir=" + path.Join(e.secretsABSPath),
				"--client-ca-file=" + path.Join(e.secretsABSPath, "apiserver.issuing_ca"),
				// TODO dedicated certs
				"--tls-cert-file=" + path.Join(e.secretsABSPath, "apiserver.certificate"),
				"--tls-private-key-file=" + path.Join(e.secretsABSPath, "apiserver.private_key"),

				"--read-only-port=0",
				"--anonymous-auth=false",

				"--authentication-token-webhook",
				"--authentication-token-webhook-cache-ttl=5s",
				"--authorization-mode=Webhook",

				"--cni-conf-dir=" + e.networkABSPath, // no-op if
				"--cni-bin-dir=" + e.binABSPath,      // --network-plugin is unset
				networkPluginArgs,

				"--cadvisor-port=0",
				"--cgroups-per-qos=true", // TODO
				"--max-pods=110",
				"--node-ip=" + e.outboundIP.String(),
				"--node-labels=e2e=mononode",
				"--application-metrics-count-limit=50",
			},
				" "),
		},
		{
			Section: "Service",
			Name:    "Restart",
			Value:   "no",
		},
		{
			Section: customSystemdSection,
			Name:    "ProbeURL",
			Value:   fmt.Sprintf("http://127.0.0.1:10248/healthz"),
		},
	}
	sdKubelet = append(sdKubelet, e.systemdEnd2EndSection...)
	err := e.writeSystemdUnit(sdKubelet, fmt.Sprintf("%skubelet.service", config.ViperConfig.GetString("systemd-unit-prefix")))
	if err != nil {
		return err
	}
	return nil
}

func (e *Environment) createEtcdUnit() error {
	sdKubelet := []*unit2.UnitOption{
		{
			Section: "Unit",
			Name:    "Description",
			Value:   "etcd for end to end testing",
		},
		{
			Section: "Unit",
			Name:    "After",
			Value:   "network.target",
		},
		{
			Section: "Service",
			Name:    "Type",
			Value:   "notify",
		},
		{
			Section: "Service",
			Name:    "ExecStart",
			Value: strings.Join([]string{
				e.binaryEtcd.binaryABSPath,
				"--name=etcdv" + e.binaryEtcd.version,
				"--data-dir=" + e.etcdDataABSPath,
				"--auto-compaction-retention=0",
				"--quota-backend-bytes=0",
				"--metrics=basic",
				// TODO use dedicated certs
				"--ca-file=" + path.Join(e.secretsABSPath, "apiserver.issuing_ca"),
				"--cert-file=" + path.Join(e.secretsABSPath, "apiserver.certificate"),
				"--key-file=" + path.Join(e.secretsABSPath, "apiserver.private_key"),
				"--client-cert-auth=true",
				"--trusted-ca-file=" + path.Join(e.secretsABSPath, "apiserver.issuing_ca"),
				fmt.Sprintf("--listen-client-urls=http://127.0.0.1:2379,https://%s:2379", e.GetPublicIP()),
				fmt.Sprintf("--advertise-client-urls=http://127.0.0.1:2379,https://%s:2379", e.GetPublicIP()),
			},
				" "),
		},
		{
			Section: "Service",
			Name:    "Restart",
			Value:   "no",
		},
		{
			Section: customSystemdSection,
			Name:    "ProbeURL",
			Value:   fmt.Sprintf("http://127.0.0.1:2379/health"),
		},
	}
	sdKubelet = append(sdKubelet, e.systemdEnd2EndSection...)
	err := e.writeSystemdUnit(sdKubelet, fmt.Sprintf("%setcd.service", config.ViperConfig.GetString("systemd-unit-prefix")))
	if err != nil {
		return err
	}
	return nil
}

func (e *Environment) setupSystemd() error {
	conn, err := dbus.NewSystemdConnection()
	if err != nil {
		glog.Errorf("Cannot connect to dbus: %v", err)
		return err
	}
	e.dbusClient = conn

	err = e.createKubeletUnit()
	if err != nil {
		return err
	}

	err = e.createEtcdUnit()
	if err != nil {
		return err
	}

	err = conn.Reload()
	if err != nil {
		glog.Errorf("Cannot daemon-reload: %v", err)
		return err
	}
	return nil
}