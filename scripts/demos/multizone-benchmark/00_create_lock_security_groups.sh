. ./configure.sh

SOURCE_IPS=()
if [ "$1" != "" ]
then
    SOURCE_IPS=(-i $1)
fi

case "${BACKEND}" in
        "docker")
                echo "No equivalent security lock for the Docker backend"
                ;;
        "aws")
		aerolab config aws list-security-groups | grep AeroLab >/dev/null 2>&1
		if [ $? -ne 0 ]
		then
		    aerolab config aws create-security-groups
		fi
		aerolab config aws lock-security-groups ${SOURCE_IPS[@]}
                ;;
        "gcp")
		# TODO: make this not just for Jim
		aerolab config gcp list-firewall-rules | grep jdoty 
		if [ $? -ne 0 ]
		then
		    aerolab config gcp create-firewall-rules -n jdoty
		fi
		aerolab config gcp lock-firewall-rules -n jdoty
                ;;
        *)
                echo "Unknown backend"
                ;;
esac
