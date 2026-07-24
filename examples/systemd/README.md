# Systemd Unit

_These instructions mimic Debian's conventions for Prometheus exporters, specifically the `prometheus` user._

The unit file `jobstats_cgroup_exporter.service` in this directory should be
placed in `/etc/systemd/system`. The `jobstats_cgroup_exporter` binary should be
placed in `/usr/local/bin`.

The service references a defaults file located at
`/etc/default/jobstats_cgroup_exporter`. This file contains the command line
arguments needed to run the exporter. A sample is found in
`jobstats_cgroup_exporter.defaults`.

The service runs as `prometheus` with `AmbientCapabilities=CAP_SYS_PTRACE` so that it
can read other user's processes.

```console
sudo cp jobstats_cgroup_exporter.service /etc/systemd/system/
sudo cp jobstats_cgroup_exporter.defaults /etc/default/jobstats_cgroup_exporter
sudo systemctl daemon-reload
sudo systemctl enable --now jobstats_cgroup_exporter
```
