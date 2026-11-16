# Smolcorp: lets its own copy of bubblewrap make the user namespaces that sandboxed hires need. Every program
# that copy starts runs under smolcorp-bwrap-child too, which denies it every capability and any user namespace.
# Put here by the Smolcorp installer, or by the one command Smolcorp Setup shows.
abi <abi/4.0>,
include <tunables/global>

profile smolcorp-bwrap /usr/local/lib/smolcorp/bwrap flags=(attach_disconnected,mediate_deleted) {
  capability,
  userns,
  mount,
  umount,
  pivot_root,
  network,
  unix,
  signal,
  ptrace,
  mqueue,
  io_uring,
  dbus,
  /{**,} rwlkm,
  /** pix -> &smolcorp-bwrap//&smolcorp-bwrap-child,
}

profile smolcorp-bwrap-child flags=(attach_disconnected,mediate_deleted) {
  audit deny capability,
  audit deny userns,
  network,
  unix,
  signal,
  ptrace,
  mqueue,
  io_uring,
  dbus,
  /{**,} rwlkmix,
}
