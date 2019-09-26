# RHCS Configuration Exporter

The goal of this tool is to extract key elements of the Ceph configuration to a file. The file may then be used as metadata to import the cluster into rook-ceph enabling Kubernetes applications to consume storage from an external ceph cluster.

## Installation
This is a simple python script, so could be installed directly by copying the file or installing with the rpm.

## How to Use it
The ceph export command should be installed in a mgr or mon node. Once installed the command provides the following options;
```
[root@ceph-mgr ~]$ ceph-export.py -h
usage: ceph-export.py [-h] [-o OUTPUT] [-c CONFDIR] [-u USER]

Export RHCS configuration information

optional arguments:
  -h, --help            show this help message and exit
  -o OUTPUT, --output OUTPUT
                        output file for the export (default: ~/rhcs-config-
                        export.yaml)
  -c CONFDIR, --confdir CONFDIR
                        ceph configuration directory (default: /etc/ceph)
  -u USER, --user USER  Ceph user to use for the keyring (default: admin)

```
### Example output
Here's an example of the file the tool creates
```
---
fsid: 6d210768-d391-409b-b585-56d54554da8c
mon_host: 10.90.90.160
secret: AQCrGYBdRH3XLRAA+LojQqElDRXHL6FIb5QvXg==
```
  
## TODO
* Add mgr/dashboard info to provide the ability for the import process to setup link-n-launch
* add ceph version
* add service layout information
* support yaml/bash output formats (yaml only currently)
* complete packaging
