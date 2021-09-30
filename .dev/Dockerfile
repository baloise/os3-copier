FROM quay.balgroupit.com/baloise-base-images/golang:1.15.5-alpine3.12

# Copy CA Certificates for Baloise Proxy, because the interject https and replace server certificates
COPY Webgateway-PROD_CA.crt /usr/local/share/ca-certificates/Webgateway-PROD_CA.crt
COPY Zscaler_CA.crt /usr/local/share/ca-certificates/Zscaler_CA.crt
RUN update-ca-certificates

# Install net tools like netstat...
#
# RUN apt-get update \
# && apt-get install -y net-tools \
# && apt-get clean && rm -rf /var/cache/apt/* && rm -rf /var/lib/apt/lists/* && rm -rf /tmp/*

RUN export \
    http_proxy=http://185.46.212.88:9400 \
    https_proxy=http://185.46.212.88:9400 \
&& apk --no-cache upgrade \
&& apk add curl && apk add bash && apk add make && apk add gcc && apk add musl-dev

# Install Operator SDK
RUN export \
        http_proxy=http://185.46.212.88:9400 \
        https_proxy=http://185.46.212.88:9400 \
&& cd /tmp \
&& curl -Lo operator-sdk -k https://github.com/operator-framework/operator-sdk/releases/download/v0.18.2/operator-sdk-v0.18.2-x86_64-linux-gnu \
#&& curl -Lo operator-sdk -k https://github.com/operator-framework/operator-sdk/releases/download/v1.4.2/operator-sdk_linux_amd64 \
&& chmod +x operator-sdk \
&& mv operator-sdk /usr/local/bin/
# Install OpenShift client oc
RUN curl https://occlient-download-openshift-client-download.shapp.os1.balgroupit.com/download/linux/oc -o /usr/local/bin/oc \
&& chmod +x /usr/local/bin/oc

