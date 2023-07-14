#!/bin/bash

set -e
. ./configure.sh

#### create cluster ####

AZS=""
BACKEND_OPS=()

case "${BACKEND}" in
"aws")
  if [ ${#AWS_AVAILABILITY_ZONES[@]} -lt 1 ]; then
    echo "AWS_AVAILABILITY_ZONES must have at least one AZ, for example: AWS_AVAILABILITY_ZONES=(us-east-1a us-east-1b)"
    exit 1
  fi
  AZS=("${AWS_AVAILABILITY_ZONES[@]}")
  BACKEND_OPS=("--instance-type ${CLUSTER_AWS_INSTANCE}" "--ebs=${AWS_EBS}")
  echo "Creating cluster, ${NODES_PER_AZ} nodes per AZ, AZs=${AZS[@]}"
  ;;
"gcp")
  if [ ${#GCP_AVAILABILITY_ZONES[@]} -lt 1 ]; then
    echo "GCP_AVAILABILITY_ZONES must have at least one AZ, for example: GCP_AVAILABILITY_ZONES=(us-central1-a us-central1-b)"
    exit 1
  fi
  AZS=("${GCP_AVAILABILITY_ZONES[@]}")
  BACKEND_OPS=("--firewall=jdoty" "--instance ${CLUSTER_GCP_INSTANCE}" "--disk ${GCP_ROOT_VOLUME}")
  for data_volume in "${GCP_DATA_VOLUMES[@]}"; do
    BACKEND_OPS+=("--disk ${data_volume}")
  done
  echo "Creating cluster, ${NODES_PER_AZ} nodes per AZ, AZs=${AZS[@]}"
  ;;
"docker")
  AZS=""
  BACKEND_OPS=()
  echo "Create cluster in docker, total nodes=$((${#AWS_AVAILABILITY_ZONES[@]} * ${NODES_PER_AZ}))"
  ;;
*)
  echo "Unknown aerolab backend"
  ;;
esac

sed "s/_NAMESPACE_/${NAMESPACE}/g" ${TEMPLATE} >aerospike.conf
STAGE="create"
START_NODE=0
END_NODE=0
RACK_NO=0
for i in ${AZS[@]}; do
  START_NODE=$((${END_NODE} + 1))
  END_NODE=$((${START_NODE} + ${NODES_PER_AZ} - 1))
  RACK_NO=$((${RACK_NO} + 1))
  nodes=""
  for j in $(seq ${START_NODE} ${END_NODE}); do
    [ "${nodes}" = "" ] && nodes=${j} || nodes="${nodes},${j}"
  done

  AZ=""
  if [ "${BACKEND}" = "aws" ]; then
    AZ="--subnet-id=${i}"
  else
    AZ="--zone=${i}"
  fi

  aerolab cluster ${STAGE} \
    -n ${CLUSTER_NAME} \
    -c ${NODES_PER_AZ} \
    -v ${VER} \
    -o aerospike.conf \
    --start=n \
    ${AZ} ${BACKEND_OPS[@]}

  aerolab conf rackid \
    -n ${CLUSTER_NAME} \
    -l ${nodes} \
    -i ${RACK_NO} \
    -m ${NAMESPACE} \
    -r -e

  STAGE="grow"
done

rm -f aerospike.conf

#### partition disks ####
echo "Partitioning disks"
if [ "${BACKEND}" != "docker" ]; then
  aerolab cluster partition create -n ${CLUSTER_NAME} -p ${DATA_PARTITIONS}
  aerolab cluster partition conf -n ${CLUSTER_NAME} --namespace=${NAMESPACE} --configure=device
fi

#### start cluster ####
echo "Starting cluster"
aerolab aerospike start -n ${CLUSTER_NAME} -l all

#### let the cluster do it's thang ####
echo "Wait"
sleep 15

#### setup security ####
echo "Security"
aerolab attach asadm -n ${CLUSTER_NAME} -- -U admin -P admin -e "enable; manage acl create role superuser priv read-write-udf"
aerolab attach asadm -n ${CLUSTER_NAME} -- -U admin -P admin -e "enable; manage acl grant role superuser priv sys-admin"
aerolab attach asadm -n ${CLUSTER_NAME} -- -U admin -P admin -e "enable; manage acl grant role superuser priv user-admin"
aerolab attach asadm -n ${CLUSTER_NAME} -- -U admin -P admin -e "enable; manage acl create user superman password krypton roles superuser"

#### copy astools ####
echo "Copy astools"
aerolab files upload -n ${CLUSTER_NAME} astools.conf /etc/aerospike/astools.conf

#### apply roster
echo "SC-Roster"
RET=1
while [ ${RET} -ne 0 ]; do
  aerolab roster apply -m ${NAMESPACE} -n ${CLUSTER_NAME}
  RET=$?
  [ ${RET} -ne 0 ] && sleep 10
done

#### exporter ####
echo "Adding exporter"
aerolab cluster add exporter -n ${CLUSTER_NAME} -o ape.toml

#### deploy ams ####
echo "AMS"
case "${BACKEND}" in
"aws")
  aerolab client create ams -n ${AMS_NAME} -s ${CLUSTER_NAME} --instance-type ${AMS_AWS_INSTANCE} --ebs=40 --subnet-id=${AZS[0]}
  ;;
"gcp")
  aerolab client create ams -n ${AMS_NAME} -s ${CLUSTER_NAME} --instance ${AMS_GCP_INSTANCE} --disk pd-balanced:100 --zone=${AZS[0]} --firewall=jdoty
  ;;
"docker")
  aerolab client create ams -n ${AMS_NAME} -s ${CLUSTER_NAME}
  ;;
*)
  echo "Unknown aerolab backend"
  ;;
esac
echo
aerolab client list | grep ${AMS_NAME}
