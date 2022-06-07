#!/usr/bin/env bash
set -eu

echo ${0}
echo $@

ods_git_ref=
registry_username=
registry_token=

while [[ "$#" -gt 0 ]]; do
  case $1 in

    --branch) ods_git_ref="$2"; shift;;
    --registry_username) registry_username="$2"; shift;;
    --registry_token) registry_token="$2"; shift;;

esac; shift; done

registry_username="${registry_username:-null}"
registry_token="${registry_token:-null}"
ods_git_ref="${ods_git_ref:-master}"

echo " "
echo "bootstrap.sh: Will build ods box against git-ref ${ods_git_ref}"
echo "bootstrap.sh: Credentials for registry: ${registry_username} // ${registry_token} "

echo " "
echo "--------------------------------------------------------------------------------------------------------------"
echo "bootstrap.sh: Showing current ssh passwords. They might be needed to connect to instance for debugging errors."
echo "--------------------------------------------------------------------------------------------------------------"
ls -1a ${HOME}/.ssh | grep -v "^\.\.*$" | \
    while read -r file; do echo " "; echo ${file}; echo "------------"; cat ${HOME}/.ssh/${file} || true; done
echo " "
echo "--------------------------------------------------------------------------------------------------------------"
echo " "
sleep 2

# install modern git version as required by repos.sh
sudo yum update -y || true
sudo yum install -y yum-utils epel-release https://repo.ius.io/ius-release-el7.rpm || true
sudo yum -y install https://packages.endpointdev.com/rhel/7/os/x86_64/endpoint-repo.x86_64.rpm || true
sudo yum -y install git gitk iproute lsof xrdp tigervnc-server remmina firewalld git2u-all glances golang jq tree \
            etckeeper unzip \
            adoptopenjdk-8-hotspot adoptopenjdk-11-hotspot adoptopenjdk-8-hotspot-jre adoptopenjdk-11-hotspot-jre \
            || true

opendevstack_dir="${HOME}/opendevstack"
mkdir -pv "${opendevstack_dir}"
cd "${opendevstack_dir}" || return
curl -sSLO https://raw.githubusercontent.com/opendevstack/ods-core/${ods_git_ref}/scripts/repos.sh
chmod u+x ./repos.sh
./repos.sh --git-ref "${ods_git_ref}" --verbose

echo " "
echo "----------------------------------------------------"
echo "bootstrap.sh: Deploy in instance the ODS environment"
echo "----------------------------------------------------"
echo " "
sleep 2

set +x
cd ods-core
time bash ods-devenv/scripts/deploy.sh --branch "${ods_git_ref}" --target basic_vm_setup \
    --registry_username "${registry_username}" --registry_token "${registry_token}"
set -x
