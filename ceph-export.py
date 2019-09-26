import os
import sys
import yaml
import argparse
import configparser

KEYRING_FILE = 'ceph.client.{}.keyring'


class Defaults(object):
    user = "admin"
    conf_dir = '/etc/ceph'
    output = '~/rhcs-config-export.yaml'


def ready():
    if not os.path.isfile(os.path.join(args.confdir, 'ceph.conf')):
        return False, "ceph.conf not found"
    if not os.path.isfile(os.path.join(args.confdir, 
                          KEYRING_FILE.format(args.user))) and \
      not os.path.isfile(os.path.join(args.confdir, 'keyring-store', 'keyring')):
        return False, "missing keyring"
    return True, ""

def get_config(filename):
    conf = configparser.ConfigParser()
    conf.read(filename)
    return conf

def find_keyring():
    conf = None

    ceph_conf_keyring = os.path.join(args.confdir, KEYRING_FILE.format(args.user))
    ceph_conf_keystore = os.path.join(args.confdir, 'keyring-store', 'keyring')
    if os.path.isfile(ceph_conf_keyring):
        conf = get_config(ceph_conf_keyring)
    elif os.path.isfile(ceph_conf_keystore):
        conf = get_config(ceph_conf_keystore)

    if not conf or not conf.sections():
        # conf empty or can't read it!
        return None
    if 'client.admin' not in conf.sections():
        # no client.admin section
        return None
    if 'key' not in conf['client.' + args.user].keys():
        # no key in client.admin section
        return None

    return conf['client.' + args.user]['key']

def abort(message=None):
    error_str = ": {}".format(message) if message else ""
    print("Unable to continue{}".format(error_str))
    sys.exit(1)

def help():
    print("help me...")

def parse_args():
    parser = argparse.ArgumentParser(description="Export RHCS configuration information",
                                     formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument("-o", "--output", type=str, required=False,
                        default=Defaults.output, help="output file for the export")
    parser.add_argument("-c", "--confdir", type=str, required=False,
                        default=Defaults.conf_dir, help="ceph configuration directory")
    parser.add_argument("-u", "--user", type=str, required=False,
                        default=Defaults.user, help="Ceph user to use for the keyring")
    return parser.parse_args()

def main():

    ok, _err = ready()
    if not ok:
        abort(_err)

    conf = get_config(os.path.join(args.confdir, 'ceph.conf'))
    if not all([i in conf['global'].keys() for i in ['fsid', 'mon host']]):
        abort("Invalid ceph.conf file detected. Missing fsid and/or mon host")

    keyring = find_keyring()
    if not keyring:
        abort("Can't find a keyring to use")

    settings = dict()
    mon_host_str = conf['global']['mon host']
    settings['fsid'] = conf['global']['fsid']
    settings['mon_host'] = mon_host_str.split(',')[0].split(':')[1]
    settings['secret'] = keyring

    output = yaml.safe_dump(settings, indent=2, explicit_start=2)

    try:
        with open(os.path.expanduser(args.output), 'w') as f:
            f.write(output)
    except:
        print("Unexpected problem writing the file, dumping config here")
        print(output)
    else:
        print("Config written to {}".format(os.path.expanduser(args.output)))

if __name__ == '__main__':
    args = parse_args()
    main()