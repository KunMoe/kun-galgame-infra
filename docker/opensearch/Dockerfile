# Catalog search engine: OpenSearch + the three analysis plugins the catalog
# mapping requires (analysis chain: apps/api/internal/platform/catalog/search/
# osengine/analysis.json). The infinilabs pinyin plugin ships one build per
# OpenSearch PATCH version — bumping OS_VERSION without a matching pinyin
# release makes the install fail at build time, which is the point: a pluginless
# image would boot and silently build a pinyin-less index (refused at runtime
# too by CheckPlugins, but failing here is cheaper).
ARG OS_VERSION=2.19.6
# public.ecr.aws over docker.io: the docker.io mirror times out from this infra.
FROM public.ecr.aws/opensearchproject/opensearch:${OS_VERSION}
ARG OS_VERSION
RUN /usr/share/opensearch/bin/opensearch-plugin install --batch analysis-icu \
    && /usr/share/opensearch/bin/opensearch-plugin install --batch analysis-kuromoji \
    && /usr/share/opensearch/bin/opensearch-plugin install --batch \
        "https://get.infini.cloud/opensearch/analysis-pinyin/${OS_VERSION}"
