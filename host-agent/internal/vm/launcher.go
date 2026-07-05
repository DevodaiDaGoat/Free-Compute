package vm

// QEMU launch configuration with security hardening.
// Each VM runs in a sandboxed environment:
// - Separate network namespace (no direct host network access)
// - cgroup resource limits (CPU, memory, I/O)
// - Seccomp filtering on QEMU process
// - No access to host filesystem outside /var/lib/freecompute/vms/<vm_id>/

// Example QEMU command (generated at runtime):
//
// qemu-system-x86_64 \
//   -name <vm_name> \
//   -machine type=q35,accel=kvm \
//   -cpu host \
//   -smp <cpu_cores> \
//   -m <ram_mb> \
//   -drive file=<disk_image>,format=qcow2,if=virtio \
//   -netdev user,id=net0,restrict=y \
//   -device virtio-net-pci,netdev=net0 \
//   -display none \
//   -vnc unix:/tmp/vm-<id>.sock \
//   -sandbox on,obsolete=deny,elevateprivileges=deny,spawn=deny,resourcecontrol=deny \
//   -monitor unix:/tmp/vm-<id>-monitor.sock,server,nowait

// TODO: Implement QEMU process launching with the above security flags
