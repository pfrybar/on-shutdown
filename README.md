## on-shutdown

Run a command before system shutdown (or restart) on any Linux machine that uses systemd.

This tool works with [systemd inhibitor locks](https://systemd.io/INHIBITOR_LOCKS/) to
ensure the given command runs before any services even begin shutting down.

### Usage

```
on-shutdown <command> [options...]
```

Example 1: broadcast a message before shutdown
```
on-shutdown wall "Hello from on-shutdown!"
```

Example 2: send HTTP request before shutdown (in the background)
```
nohup on-shutdown curl https://some.domain/some-endpoint &
```

### Integrate with systemd

`on-startup` can be run as a systemd service to automatically execute a command at
every shutdown.

#### Example

1. Create a systemd unit called `/etc/systemd/system/http-shutdown.service`:
```
[Unit]
Description=Run HTTP request before shutdown

[Service]
ExecStart=/usr/loca/bin/on-shutdown curl https://some.domain/some-endpoint

[Install]
WantedBy=multi-user.target
```

2. `sudo systemctl daemon-reload`
3. `sudo systemctl enable http-shutdown.service`
4. `sudo systemctl start http-shutdown.service`

Now whenever the system is shutdown or restarted, the HTTP request will be made.

### Timeouts and InhibitDelayMaxSec

When using systemd inhibitor locks to delay system shutdown, there is a limit to how long
a lock can be held. This value is known as the
[InhibitDelayMaxSec](https://www.freedesktop.org/software/systemd/man/latest/logind.conf.html#InhibitDelayMaxSec=).
If a lock is held for longer than this value, the shutdown sequence will be started.
When using `on-shutdown`, if the command takes longer than InhibitDelayMaxSec, it will
be killed by the system. This will be detected and an error message will be printed:

```
Error: command did not complete within the InhibitDelayMaxSec (5s) and was killed - consider increasing this value:
https://www.freedesktop.org/software/systemd/man/latest/logind.conf.html#InhibitDelayMaxSec=
```

### Rationale

While this tool makes it easy to run a command before a system begins to shuts down, there
are other ways to achieve this as well as explained in
[this Stack Exchange post](https://unix.stackexchange.com/questions/48973/execute-a-command-before-shutdown/294539#294539).
Another possible option is to create a custom systemd target which runs _after_
`multi-user.target` and use this to run commands at the first part of system shutdown.

I initially tried these options for my use case: to drain a Kubernetes node when the
node shuts down or restarts. For this, the ordering is very important - I need to run
`kubectl drain ...` _before_ any of the Kubernetes components begin shutting down and
_before_ containerd stops the containers. I also need to make sure my Kubernetes CSI
pods are the last to shutdown in order to properly close the storage interfaces.

After many failed attempts, I came across systemd inhibitor locks and decided to write a
tool which would guarantee that the nodes would be drained before any services began shutting
down. I then decided to make it generic and run any user-defined command :)

### Reference

Heavily inspired by [this post](https://trstringer.com/systemd-inhibitor-locks/) which explains
how to use systemd inhibitor locks in Go.
