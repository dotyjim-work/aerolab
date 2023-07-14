set -e
. ./configure.sh
# create clients
case "${BACKEND}" in
"aws")
  if [ ${#AWS_AVAILABILITY_ZONES[@]} -lt 1 ]; then
    echo "AWS_AVAILABILITY_ZONES must have at least one AZ, for example: AWS_AVAILABILITY_ZONES=(us-east-1a us-east-1b)"
    exit 1
  fi
  AZS=("${AWS_AVAILABILITY_ZONES[@]}")
  BACKEND_OPS=("--instance-type ${CLIENT_AWS_INSTANCE}" "--ebs=${AWS_EBS}")
  echo "Creating clients, ${CLIENTS_PER_AZ} nodes per AZ, AZs=${AZS[@]}"
  ;;
"gcp")
  if [ ${#GCP_AVAILABILITY_ZONES[@]} -lt 1 ]; then
    echo "GCP_AVAILABILITY_ZONES must have at least one AZ, for example: GCP_AVAILABILITY_ZONES=(us-central1-a us-central1-b)"
    exit 1
  fi
  AZS=("${GCP_AVAILABILITY_ZONES[@]}")
  BACKEND_OPS=("--firewall=jdoty" "--instance ${CLIENT_GCP_INSTANCE}" "--disk ${GCP_ROOT_VOLUME}")
  echo "Creating clients, ${CLIENTS_PER_AZ} nodes per AZ, AZs=${AZS[@]}"
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

echo "Creating clients"
set -e
STAGE="create"
for i in ${AZS[@]}; do

  AZ=""
  if [ "${BACKEND}" = "aws" ]; then
    AZ="--subnet-id=${i}"
  else
    AZ="--zone=${i}"
  fi

  aerolab client ${STAGE} tools \
    -n ${CLIENT_NAME} \
    -c ${CLIENTS_PER_AZ} \
    -v ${VER} \
    ${AZ} ${BACKEND_OPS[@]}
  STAGE="grow"
done

# copy astools
echo "Copy astools"
aerolab files upload -c -n ${CLIENT_NAME} astools.conf /etc/aerospike/astools.conf || exit 1

# configure asbench monitoring
aerolab client configure tools -l all -n ${CLIENT_NAME} --ams ${AMS_NAME}
