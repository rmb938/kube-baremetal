# -*- mode: Python -*-

# set defaults

kind_cluster_name = "kube-baremetal"
kind_kubeconfig = "kind-kubeconfig"

settings = {}

# global settings
settings.update(read_json(
    "tilt-settings.json",
    default={},
))

tilt_helper_dockerfile_header = """
# Tilt image
FROM alpine:3.11 as tilt-helper
# Support live reloading with Tilt
RUN wget --output-document /restart.sh --quiet https://raw.githubusercontent.com/windmilleng/rerun-process-wrapper/master/restart.sh  && \
    wget --output-document /start.sh --quiet https://raw.githubusercontent.com/windmilleng/rerun-process-wrapper/master/start.sh && \
    chmod +x /start.sh && chmod +x /restart.sh
"""


def deploy_baremetal_manager():
    tilt_dockerfile_header_manager = """
FROM alpine:3.11 as tilt
WORKDIR /
COPY --from=tilt-helper /start.sh .
COPY --from=tilt-helper /restart.sh .
COPY .tiltbuild/manager .
COPY discovery_files /discovery_files
"""

    # Set up a local_resource build of the provider's manager binary. The provider is expected to have a main.go in
    # manager_build_path. The binary is written to .tiltbuild/manager.
    local_resource(
        "manager",
        cmd='mkdir -p .tiltbuild;CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -ldflags \'-extldflags "-static"\' -o .tiltbuild/manager main.go',
        deps=[
            "main.go",
            "go.mod",
            "go.sum",
            "api",
            "apis",
            "controllers",
            "webhook",
            "webhooks",
            "pkg",
            "discovery_files"
        ],
    )

    dockerfile_contents_manager = "\n".join([
        tilt_helper_dockerfile_header,
        tilt_dockerfile_header_manager
    ])

    docker_build(
        ref='controller',
        context='.',
        dockerfile_contents=dockerfile_contents_manager,
        target="tilt",
        entrypoint="sh /start.sh /manager",
        only=[".tiltbuild/manager", "discovery_files"],
        live_update=[
            sync("discovery_files", "/discovery_files"),
            sync(".tiltbuild/manager", "/manager"),
            run("sh /restart.sh"),
        ],
    )

    yaml = str(kustomize("config/default"))
    substitutions = settings.get("kustomize_substitutions", {})
    for substitution in substitutions:
        value = substitutions[substitution]
        yaml = yaml.replace("${" + substitution + "}", value)
    k8s_yaml(blob(yaml))

    k8s_resource('kube-baremetal-controller-manager', port_forwards='0.0.0.0:8081:8081')


# Prepull all the cert-manager images to your local environment and then load them directly into kind. This speeds up
# setup if you're repeatedly destroying and recreating your kind cluster, as it doesn't have to pull the images over
# the network each time.
def deploy_cert_manager():
    registry = "quay.io/jetstack"
    version = "v0.11.0"
    images = ["cert-manager-controller", "cert-manager-cainjector", "cert-manager-webhook"]

    if settings.get("preload_images_for_kind"):
        for image in images:
            local("docker pull {}/{}:{}".format(registry, image, version))
            local("kind load docker-image --name {} {}/{}:{}".format(kind_cluster_name, registry, image, version))

    local(
        "kubectl apply --kubeconfig {} -f https://github.com/jetstack/cert-manager/releases/download/{}/cert-manager.yaml".format(
            kind_kubeconfig, version))

    # wait for the service to become available
    local(
        "kubectl wait --kubeconfig {} --for=condition=Available --timeout=300s apiservice v1beta1.webhook.cert-manager.io".format(
            kind_kubeconfig))


# Users may define their own Tilt customizations in tilt.d. This directory is excluded from git and these files will
# not be checked in to version control.
def include_user_tilt_files():
    user_tiltfiles = listdir("tilt.d")
    for f in user_tiltfiles:
        include(f)


##############################
# Actual work happens here
##############################
include_user_tilt_files()

deploy_cert_manager()

deploy_baremetal_manager()
