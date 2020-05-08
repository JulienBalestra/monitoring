# dd-wrt

Intend to setup and monitor a dd-wrt router.

## dd-wrt setup

### ssh-keys

In the **service** tab, under the *Secure shell* section:

* SSHd : Enable
* Password Login : Disable

Add any **Authorized Keys**.

Get from any host the current loaded public keys:
```shell script
ssh-add -L
```

### configuration list

**service** / **dnsmasq**

There is a text box for __Additional Dnsmasq Options__:
```text
server=8.8.8.8
server=8.8.4.4
log-queries
log-facility=/tmp/dnsmasq.log
```

**service** / **USB**:

Core USB Support - Enable
USB Storage Support - Enable 


**command**:

```shell script
/tmp/mnt/sda2/monitoring.startup
``` 


## build

```shell script
make arm amd64
```
