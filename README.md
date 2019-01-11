Switch Firmware: Service Management for configuration and data
==========================================================

Switch core service responsible for:
* Getting configuration from the GTB server
* Installing/Removing/Starting switch services
* Control drivers
* Control groups
* Agregating switch status and send it back to the GTB server

Build Requirement: 
* golang-go > 1.9
* glide
* devscripts
* make

Run dependancies:
* rethindkb
* mosquitto

To compile it:
* GOPATH needs to be configured, for example:
```
    export GOPATH=$HOME/go
```

* Install go dependancies:
```
    make prepare
```

* To clean build tree:
```
    make clean
```

* Multi-target build:
```
    make all
```

* To build x86 target:
```
    make bin/energieip-swh200-firmware-amd64
```

* To build armhf target:
```
    make bin/energieip-swh200-firmware-armhf
```
* To create debian archive for x86:
```
    make deb-armhf
```

* To install debian archive on the target:
```
    scp build/*.deb <login>@<ip>:~/
    ssh <login>@<ip>
    sudo dpkg -i *.deb
```

For development:
* recommanded logger: *rlog*
* For dependency: use *common-components-go* library
