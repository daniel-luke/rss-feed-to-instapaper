### Update feed config
Delete the configmap and then:
```shell
kubectl create configmap rss-syncer-config --from-file=config.yaml=config.yaml -n rss-feed-to-instapaper
```
