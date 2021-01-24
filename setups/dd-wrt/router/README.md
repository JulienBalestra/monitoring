# dd-wrt

Intend to setup and monitor a dd-wrt router.

Running use cases: 
* Netgear R6700 v3

## dd-wrt setup

### Basic setup

Configure: 
* Router name
* Hostname

Network Setup:
* Local DNS -> 8.8.8.8

Time Settings:
* Time Zone -> **greenwich**
* Server IP/Name -> 0.fr.pool.ntp.org

### Wireless

* configure dual channel 2.4/5Ghz
* Wireless Network Name (SSID)

Wireless security:

* WPA Shared Key

### dnsmasq

There is a text box for __Additional Dnsmasq Options__:
```text
server=8.8.8.8
server=8.8.4.4
log-queries
log-facility=/tmp/dnsmasq.log
```

### ssh-keys

In the **service** tab, under the *Secure shell* section:

* SSHd : Enable
* Password Login : Disable

Add any **Authorized Keys**.

Get from any host the current loaded public keys:
```shell script
ssh-add -L
```

Others:

* Disable telnet
* Disable ttraff Daemon

**service** / **USB**:

Core USB Support - Enable
USB Storage Support - Enable 
Automatic Drive Mount - Enable

**administration**

Router Management:

* Web Access Protocol -> HTTPS
* Enable Info Site -> Disable
* Remote Access -> SSH Management -> Enable
* SSH Remote Port -> 8222


**command**:

```shell script
until /tmp/mnt/sda1/monitoring.sh;do sleep 1;done
``` 


## build

```shell script
make arm && scp monitoring-arm root@192.168.1.1:/tmp/mnt/sda1/monitoring
```
