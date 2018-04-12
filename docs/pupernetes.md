## pupernetes

Use this command to manage a Kubernetes testing environment

### Synopsis

Use this command to manage a Kubernetes testing environment

### Options

```
  -c, --clean string                 clean options before setup: binaries,etcd,kubectl,kubelet,manifests,mounts,network,secrets,systemd,all,none (default "etcd,mounts")
      --cni-version string           container network interface (cni) version (default "0.7.0")
      --etcd-version string          etcd version (default "3.1.11")
  -h, --help                         help for pupernetes
      --hyperkube-version string     hyperkube version (default "1.10.0")
      --kubelet-root-dir string      directory path for managing kubelet files (default "/var/lib/e2e-kubelet")
      --systemd-unit-prefix string   prefix for systemd unit name (default "e2e-")
      --vault-version string         vault version (default "0.9.5")
  -v, --verbose int                  verbose level (default 2)
```

### SEE ALSO

* [pupernetes clean](pupernetes_clean.md)	 - Clean the environment created by setup and altered by a run
* [pupernetes run](pupernetes_run.md)	 - setup and run the environment
* [pupernetes setup](pupernetes_setup.md)	 - Setup the environment

###### Auto generated by spf13/cobra on 14-Apr-2018