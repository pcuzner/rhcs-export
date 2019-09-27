import os
import sys
import yaml
import json
import shutil
import argparse
import subprocess
import configparser

KEYRING_FILE = 'ceph.client.{}.keyring'


class Defaults(object):
    user = "admin"
    conf_dir = '/etc/ceph'
    output = '~/rhcs-config-export.yaml'
    output_format = 'yaml'


class Choices(object):
    output_format = ['yaml']


def ready():
    if not shutil.which('ceph'):
        return False, "ceph command not found!"
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

def send_command(cmd):
    cmd_stream = cmd.split(' ')
    _c = subprocess.Popen(cmd_stream,
                          stdout=subprocess.PIPE,
                          stderr=subprocess.STDOUT)
    return _c.communicate()

def write_file(content):
    
    try:
        with open(os.path.expanduser(args.output), 'w') as f:
            f.write(content)
    except:
        print("Unexpected problem writing the file, dumping config here")
        print(output)
    else:
        print("Config written to {}".format(os.path.expanduser(args.output)))

def _dump_yaml(settings):
    
    output = yaml.safe_dump(settings, indent=2, explicit_start=2)
    write_file(output)

def dump(settings):
    if args.format == 'yaml':
        _dump_yaml(settings)
    else:
        abort("Unknown output format requested")    

def parse_args():
    parser = argparse.ArgumentParser(description="Export RHCS configuration information",
                                     formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument("-o", "--output", type=str, required=False,
                        default=Defaults.output, help="output file for the export")
    parser.add_argument("-c", "--confdir", type=str, required=False,
                        default=Defaults.conf_dir, help="ceph configuration directory")
    parser.add_argument("-u", "--user", type=str, required=False,
                        default=Defaults.user, help="Ceph user to use for the keyring")
    parser.add_argument("-f", "--format", type=str, required=False,
                        choices=Choices.output_format,
                        default=Defaults.output_format, 
                        help="output file format")
    return parser.parse_args()

def main():

    ok, _err = ready()
    if not ok:
        abort(_err)

    conf = get_config(os.path.join(args.confdir, 'ceph.conf'))
    keyring = find_keyring()
    if not keyring:
        abort("Can't find a keyring to use")

    print("Fetching state of the local Ceph cluster")
    ceph_out, error = send_command('ceph -s -f json')
    if error:
        abort("ceph command failed - can't determine state")
    
    ceph_version_text, error = send_command('ceph --version')
    if error:
        abort("unable to fetch ceph version!")

    ceph_state = json.loads(ceph_out)

    settings = dict()

    settings['fsid'] = ceph_state['fsid']
    settings['version'] = str(ceph_version_text).split()[2].split('-')[0]  # just want a V.R.M for the version id
    settings['secret'] = keyring
    settings['mons'] = [mon['public_addr'].split(':')[0] for mon in ceph_state['monmap']['mons']]
    settings['mgr'] = ceph_state['mgrmap']['active_addr'].split(':')[0]
   
    dashboard_url = ceph_state['mgrmap']['services'].get('dashboard', None)
    if dashboard_url:
        settings['dashboard'] = dashboard_url
   
    if ceph_state.get('servicemap', None):
        if ceph_state['servicemap']['services'].get('rgw', None):
            daemons = ceph_state['servicemap']['services']['rgw']['daemons']
            rgws = list()
            for rgw in daemons:
                if rgw == "summary":
                    continue
                frontend_str = daemons[rgw]['metadata']['frontend_config#0']
                rgws.append(frontend_str.split('port=')[1].split()[0])
            settings['rgws'] = rgws
        
    dump(settings)


if __name__ == '__main__':
    args = parse_args()
    main()