#!/usr/bin/env bash

set -eou pipefail

dockerHubNs="kshitizmichael"
apps=('edge-gateway' 'fleet-api' 'notifier' 'web' 'ingest' 'device-sim')
architecture=('arm64')

for arg in "${apps[@]}"; do
    for arch in "${architecture[@]}"; do
        image="${dockerHubNs}/${arg}:${arch}"
        docker pull $image
        # docker buildx imagetools inspect "${dockerHubNs}/${arg}:${arch}"
        # echo "${dockerHubNs}/${arg}:${arch}"
        docker run -d --name ${arg}-${arch} ${dockerHubNs}/${arg}:${arch}
        docker stop -t 10 ${arg}-${arch}
        docker inspect ${arg}-${arch} --format='exit={{.State.ExitCode}} oom={{.State.OOMKilled}}'
        oom_status=$(docker inspect ${arg}-${arch} --format='{{.State.OOMKilled}}')
        image_size=$(docker inspect $image --format '{{.Size}}')
        max_size=$((150 * 1024 * 1024))
        user=$(docker inspect ${arg}-${arch} --format='{{.Config.User}}')
        if [[ $oom_status == "true" ]]; then
            echo "OOM status not 0"
            exit 1
        else
            echo "OOM status 0 test passed"
        fi
        if  (( image_size <= max_size)) ; then
            echo "size under defined limit"
        else
            echo "size over limit"
        fi        
        if [[ $user != "root" ]]; then
            echo "user non-root, validation passed"
        else 
            echo "user is root, validation failed"
            exit 1
        fi        

        # docker logs --tail 20 ${arg}-${arch}
        docker rm ${arg}-${arch}
    done
done    


