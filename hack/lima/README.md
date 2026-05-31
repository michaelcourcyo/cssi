# Local CSSI server test environment (Lima)

The CSSI server shells out to LVM (`lvcreate`, `mkfs.*`, `mount`) and
to the Linux kernel NFS server (`nfsd`, `exportfs`). Both are
kernel-bound features — they don't run on macOS, and Docker on macOS
shares only the Docker Desktop VM's kernel, which doesn't reliably
expose the `nfsd` module. To iterate on the server realistically from
a Mac laptop, you need a real Linux VM with a real kernel.

This directory contains a [Lima](https://lima-vm.io/) config that
builds a minimal Ubuntu 24.04 VM with:

- An LVM volume group named `cssi` carved out of an 8 GiB virtual disk.
- The kernel NFS server running, with `/srv/cssi` as the export root.
- Your `$HOME` mounted read-write, so the repo on your Mac is the same
  files you build and run inside the VM — no rsync, no scp.

## Why Lima

| Option                       | Verdict |
|------------------------------|---------|
| **Docker on Mac**            | Painful. Container shares the Docker Desktop kernel, not yours. LVM needs `--privileged` + loopback files; the kernel `nfsd` module isn't reliably present and you'd have to fall back to `nfs-ganesha`, which isn't what production runs. Acceptable for one-off CI smoke tests, miserable for daily dev. |
| **Vagrant + VirtualBox**     | Works but heavier, slower to spin up, and VirtualBox on Apple Silicon has been historically rough. |
| **Lima**                     | Native Apple Silicon via QEMU's HVF acceleration, one YAML file describes the environment, host↔VM file sharing via virtio-fs (fast and writable). Minimal moving parts. |

For **CI on Linux** you wouldn't reach for Lima — you'd just
`apt-get install lvm2 nfs-kernel-server` on the runner directly. The
tests should be environment-agnostic; only the bootstrap differs.

## One-time setup

```bash
brew install lima
make vm-up        # creates and starts the VM (a few minutes on first run)
```

## Daily commands

| What                              | Command            |
|-----------------------------------|--------------------|
| Start (after first creation)      | `make vm-up`       |
| Open a shell in the VM            | `make vm-shell`    |
| Stop the VM (preserves state)     | `make vm-down`     |
| Delete the VM (wipes VG, exports) | `make vm-destroy`  |

`vm-down` preserves `/srv/cssi` and the `cssi` VG, so volumes you
created during one session are still there next time you start.
`vm-destroy` wipes everything.

## Inside the VM

The repo path on the Mac mirrors the path in the VM, so you can `cd`
to wherever you keep this checkout:

```bash
cd ~/kasten.io/github/cssi
make build-cssi-server
sudo ./bin/cssi-server --port 9000 --vg cssi
```

The VM's `:9000` is port-forwarded to `127.0.0.1:9000` on the host, so
the `cssi-driver` (running on the Mac) can dial the server directly.

## Inspecting state

From inside the VM:

```bash
vgs                     # the cssi VG exists?
lvs                     # logical volumes carved so far
showmount -e localhost  # NFS exports
exportfs -v             # exports with options
```

## Tearing down stale volumes

Until `DeleteVolume` is implemented, dev sessions can accumulate LVs.
Easiest cleanup from inside the VM:

```bash
# One at a time:
sudo umount /srv/cssi/<lvname> 2>/dev/null || true
sudo lvremove -f cssi/<lvname>

# Nuclear (wipe the whole VG and start over):
sudo vgremove -ff cssi
sudo pvremove -ff /dev/vdb
sudo pvcreate /dev/vdb
sudo vgcreate cssi /dev/vdb
```
