# Systemd Unit

_These instructions mimic Debian's conventions for Prometheus exporters._

The unit file `jobstats_cgroup_exporter.service` in this directory should be
placed in `/etc/systemd/system`. The `jobstats_cgroup_exporter` binary should be
placed in `/usr/local/bin`.

The service references a defaults file located at
`/etc/default/jobstats_cgroup_exporter`. This file contains the command line
arguments needed to run the exporter. A sample is found in
`jobstats_cgroup_exporter.defaults`.

The service runs as `root` because the exporter reads other users' cgroups and
process information from the host.

```console
sudo cp jobstats_cgroup_exporter.service /etc/systemd/system/
sudo cp jobstats_cgroup_exporter.defaults /etc/default/jobstats_cgroup_exporter
sudo systemctl daemon-reload
sudo systemctl enable --now jobstats_cgroup_exporter
```
