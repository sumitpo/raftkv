FROM fluent/fluentd:v1.18.0-debian-1.0
# FROM govtechsg/fluentd-elasticsearch
USER root
RUN apt update \
      && apt install netcat-openbsd -y \
      && gem install faraday-net_http -v 3.0.2 \
      && gem install excon -v 0.109.0 \
      && gem install faraday -v 2.8.1 \
      && gem install fluent-plugin-elasticsearch --no-document --version 5.4.3
COPY ./fluentd.conf /fluentd/etc/
USER fluent
