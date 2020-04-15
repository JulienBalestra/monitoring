# metrics project


## dd-wrt setup


### ssh-keys

In the **service** tab, under the **Secure shell* section:

* SSHd : Enable
* Password Login : Disable

Add any **Authorized Keys**.

Get from any host the current loaded public keys:
```shell script
ssh-add -L
```

### build

```shell script
GOARCH=arm GOARM=5 go build .
```

### configuration list

In the **service** tab, under the **dnsmasq** section, there is a text box for __Additional Dnsmasq Options__:
```text
server=8.8.8.8
server=8.8.4.4
log-queries
log-facility=/tmp/dnsmasq.log
```
