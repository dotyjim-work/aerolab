# absolute path to a valid features.conf file
FEATURES_FILE="/home/jdoty/src/github.com/citrusleaf/aerospike-server-enterprise/etc/features.conf"

# if backend is 'docker', total nodes is NODES_PER_AZ*count(AWS_AVAILABILITY_ZONES), total clients is CLIENTS_PER_AZ*count(AWS_AVAILABILITY_ZONES)
BACKEND="aws"

# aerospike version
VER="6.2.0.6"

# names
CLUSTER_NAME="demo"
AMS_NAME="ams"
CLIENT_NAME="clients"

# region and list of AWS AZs to deploy in; also defines number of aerospike racks
AWS_REGION="us-east-1"
AWS_AVAILABILITY_ZONES=(us-east-1c us-east-1d us-east-1f)

# number of server nodes and client machines per AZ (per rack)
NODES_PER_AZ=2
CLIENTS_PER_AZ=2

# instances - cluster instance requires type with NVMe disks
CLUSTER_AWS_INSTANCE="r6id.8xlarge"
AMS_AWS_INSTANCE="m6i.2xlarge"
CLIENT_AWS_INSTANCE="c6in.8xlarge"

# namespace name
NAMESPACE="test"

# size of the root volume
AWS_EBS=40

# partitions to create per NVMe if on AWS, split as percentages
AWS_PARTITIONS=25,25,25,25

# template file name
TEMPLATE="template.conf"

# number of asbench per client instance/container - insert and read/update load
asbench_per_instance_insert=25
asbench_per_instance_load=25

# asbench details
asbench_start_key=0
asbench_end_key=1000000
asbench_threads=16
asbench_bin_name="testbin"
asbench_ru_runtime=86400
asbench_object="I1"
asbench_ru_throughput=10000
asbench_ru_percent=80 # 80 means 80 percent reads, 20 percent writes
asbench_socket_timeout=200
asbench_total_timeout=1000
asbench_retries=2
asbench_read_policy="allowReplica"

# name of config file that will be created for aerolab
export AEROLAB_CONFIG_FILE="multizone.aerolab.conf"

# setup the configs, do not modify this
. ./configure_set.sh
setsys
