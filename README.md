# mikrotik-backup
Backup mikrotik config

## Build

```shell
go mod download
go build
```

## Configure

#### Generate ssh key
```shell
ssh-keygen -f id_rsa_mikrotik
```

#### Setup mikrotik

1. Upload the public key (_id_rsa_mikrotik.pub_) to mikrotik
2. Create an user and import the public key
```
/user group add name=backup policy=ssh,read,policy,test,sensitive
/user add address=192.168.0.0/24 group=backup name=backuper
/user ssh-keys import public-key-file=id_rsa_mikrotik user=backuper
```

#### Setup a git repository
Create a repository, set write permission for mikrotik public key

#### Modify the config file

Change the config.ini file


## Run

```shell
mikrotik-backup -f config.ini
```
