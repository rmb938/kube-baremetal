ARG ARCH="amd64"
ARG OS="linux"
FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest

ARG ARCH="amd64"
ARG OS="linux"
COPY bin/kube-baremetal-manager-${OS}-${ARCH} /bin/kube-baremetal-manager
COPY discovery_files /discovery_files
USER 1000:1000

ENTRYPOINT [ "/bin/kube-baremetal-manager" ]
