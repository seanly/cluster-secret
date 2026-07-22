variable "REGISTRY" {
  default = "seanly"
}

variable "IMAGE" {
  default = "cluster-secret"
}

variable "VERSION" {
  default = "v3.0.1"
}

variable "FIXID" {
  default = "1"
}

variable "PLATFORMS" {
  default = "linux/amd64,linux/arm64"
}

variable "ACR_REGISTRY" {
  default = "registry.cn-chengdu.aliyuncs.com"
}

group "default" {
  targets = ["image"]
}

target "image" {
  labels = {
    "cloud.opsbox.author" = "seanly"
    "cloud.opsbox.image.name" = "${IMAGE}"
    "cloud.opsbox.image.version" = "${VERSION}"
    "cloud.opsbox.image.fixid" = "${FIXID}"
  }
  dockerfile = "Dockerfile"
  context    = "./"
  platforms  = split(",", PLATFORMS)
  tags       = ["${REGISTRY}/appset:${IMAGE}-${VERSION}-${FIXID}"]
  output     = ["type=image,push=true"]
}

group "acr" {
  targets = ["cluster-secret-amd64", "cluster-secret-arm64"]
}

target "cluster-secret-amd64" {
  labels = {
    "cloud.opsbox.author" = "seanly"
    "cloud.opsbox.image.name" = "${IMAGE}"
    "cloud.opsbox.image.version" = "${VERSION}"
    "cloud.opsbox.image.fixid" = "${FIXID}"
  }
  dockerfile = "Dockerfile"
  context    = "./"
  platforms  = ["linux/amd64"]
  tags = [
    "${ACR_REGISTRY}/seanly/appset:${IMAGE}-${VERSION}-${FIXID}",
    "${ACR_REGISTRY}/seanly/appset:${IMAGE}-${VERSION}",
  ]
  output = ["type=image,push=true"]
}

target "cluster-secret-arm64" {
  labels = {
    "cloud.opsbox.author" = "seanly"
    "cloud.opsbox.image.name" = "${IMAGE}"
    "cloud.opsbox.image.version" = "${VERSION}"
    "cloud.opsbox.image.fixid" = "${FIXID}"
  }
  dockerfile = "Dockerfile"
  context    = "./"
  platforms  = ["linux/arm64"]
  tags = [
    "${ACR_REGISTRY}/seanly/appset:${IMAGE}-${VERSION}-${FIXID}-arm64",
    "${ACR_REGISTRY}/seanly/appset:${IMAGE}-${VERSION}-arm64",
  ]
  output = ["type=image,push=true"]
}
