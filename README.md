# redir

```
rxlx@rxlx-m2 ~ % redir -h
Usage of redir:
  -id string
    	ID to use in the syslog message (default "redir")
  -proto string
    	Protocol to use to send the data (default "udp")
  -size int
    	Size of the chunk to read from stdin (default 1024)
  -url string
    	URL of the syslog server (default "storage.nullferatu.com:514")
  -verbose
    	Verbose mode

# examples
iostat 5 5000 | redir -id $(hostname)
cat /proc/cpuinfo | redir -url 192.168.86.10:514 -size 4096
```
