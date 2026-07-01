#!/usr/bin/env python3
"""Audit a WAYLAND_DEBUG=1 log for resize-wobble commit hazards.

Capture a log while reproducing the problem (drag a window edge for a few
seconds), then run this script over it:

    WAYLAND_DEBUG=1 ./yourapp 2> /tmp/wl.log
    python3 tools/wayland_resize_trace.py /tmp/wl.log

For every commit of the toplevel (xdg) surface it reports the attached buffer
size against the pending wp_viewport destination and xdg window geometry, and
flags the two client-side mechanisms that make a compositor scale or bounce
the window during interactive resize:

  BARE-COMMIT  size state (viewport destination / window geometry) changed
               since the previous commit, but no new buffer was attached: the
               compositor applies the new size to the old buffer.
  MISMATCH     a buffer was attached whose size disagrees with the committed
               window geometry (and destination, when a viewport is active).

A clean log (no flags) during visible trembling means the remaining wobble is
compositor-side, not client-side.
"""

import re
import sys

MSG = re.compile(
    r"^\[\s*(\d+[.,]\d+)\]\s*(->\s*)?([a-zA-Z0-9_]+)@(\d+)\.([a-zA-Z0-9_]+)\((.*)\)\s*$"
)
NEW_ID = re.compile(r"new id ([a-zA-Z0-9_]+)@(\d+)")


def ints(args):
    return [int(x) for x in re.findall(r"(?<![@\w])-?\d+", args)]


def main(path):
    stream = open(path, errors="replace") if path != "-" else sys.stdin

    buffers = {}          # wl_buffer id -> (w, h)
    viewport_surface = {} # wp_viewport id -> wl_surface id
    xdg_surface = {}      # xdg_surface id -> wl_surface id
    toplevels = set()     # wl_surface ids that have an xdg_surface
    state = {}            # wl_surface id -> pending/current commit state
    last_configure = None # latest xdg_toplevel.configure (w, h)
    flags = 0
    commits = 0

    def surf(sid):
        return state.setdefault(
            sid,
            {
                "buffer": None, "attached": False,
                "dest": None, "dest_dirty": False,
                "geom": None, "geom_dirty": False,
                "viewport": False,
            },
        )

    for line in stream:
        m = MSG.match(line.strip())
        if not m:
            continue
        ts, sent, iface, oid, op, args = m.groups()
        oid = int(oid)

        if iface == "zwp_linux_buffer_params_v1" and op in ("create", "create_immed"):
            nid = NEW_ID.search(args)
            size = ints(args)
            if op == "create_immed" and nid and len(size) >= 3:
                buffers[int(nid.group(2))] = (size[1], size[2])
            elif len(size) >= 2:
                buffers.setdefault("pending_params_%d" % oid, (size[0], size[1]))
        elif iface == "zwp_linux_buffer_params_v1" and op == "created":
            nid = NEW_ID.search(args)
            pend = buffers.pop("pending_params_%d" % oid, None)
            if nid and pend:
                buffers[int(nid.group(2))] = pend
        elif iface == "wl_shm_pool" and op == "create_buffer":
            nid = NEW_ID.search(args)
            size = ints(args)
            if nid and len(size) >= 4:
                buffers[int(nid.group(2))] = (size[1], size[2])
        elif iface == "wp_viewporter" and op == "get_viewport":
            nid = NEW_ID.search(args)
            target = re.search(r"wl_surface@(\d+)", args)
            if nid and target:
                sid = int(target.group(1))
                viewport_surface[int(nid.group(2))] = sid
                surf(sid)["viewport"] = True
        elif iface == "xdg_wm_base" and op == "get_xdg_surface":
            nid = NEW_ID.search(args)
            target = re.search(r"wl_surface@(\d+)", args)
            if nid and target:
                xdg_surface[int(nid.group(2))] = int(target.group(1))
                toplevels.add(int(target.group(1)))
        elif iface == "wp_viewport" and op == "set_destination":
            sid = viewport_surface.get(oid)
            if sid is not None:
                s = surf(sid)
                s["dest"], s["dest_dirty"] = tuple(ints(args)[:2]), True
        elif iface == "xdg_surface" and op == "set_window_geometry":
            sid = xdg_surface.get(oid)
            if sid is not None:
                g = ints(args)
                s = surf(sid)
                s["geom"], s["geom_dirty"] = (g[2], g[3]), True
        elif iface == "xdg_toplevel" and op == "configure" and not sent:
            size = ints(args)
            if len(size) >= 2 and size[0] and size[1]:
                last_configure = (size[0], size[1])
        elif iface == "wl_surface" and op == "attach":
            buf = re.search(r"wl_buffer@(\d+)", args)
            s = surf(oid)
            s["buffer"] = buffers.get(int(buf.group(1))) if buf else None
            s["attached"] = True
        elif iface == "wl_surface" and op == "commit" and oid in toplevels:
            s = surf(oid)
            commits += 1
            notes = []
            if (s["dest_dirty"] or s["geom_dirty"]) and not s["attached"]:
                notes.append("BARE-COMMIT")
            if s["attached"] and s["buffer"] and s["geom"]:
                bw, bh = s["buffer"]
                gw, gh = s["geom"]
                # Without a viewport the buffer is the surface (scale 1); with
                # one, the destination is. Either way geometry must agree with
                # the surface-size source or the compositor scales/mis-anchors.
                expect = s["dest"] if s["viewport"] and s["dest"] else (bw, bh)
                if (gw, gh) != expect:
                    notes.append("MISMATCH geom=%dx%d vs %dx%d" % (gw, gh, *expect))
            line_out = "[%s] commit wl_surface@%d buf=%s dest=%s geom=%s cfg=%s%s" % (
                ts, oid,
                "%dx%d" % s["buffer"] if s["attached"] and s["buffer"] else
                ("new" if s["attached"] else "none"),
                "%dx%d" % s["dest"] if s["dest"] else "-",
                "%dx%d" % s["geom"] if s["geom"] else "-",
                "%dx%d" % last_configure if last_configure else "-",
                ("   <== " + ", ".join(notes)) if notes else "",
            )
            if notes:
                flags += 1
                print(line_out)
            elif "-v" in sys.argv:
                print(line_out)
            s["attached"] = False
            s["dest_dirty"] = False
            s["geom_dirty"] = False

    print("\n%d toplevel commits, %d flagged." % (commits, flags))
    if flags == 0 and commits:
        print("Client commit stream is consistent; residual wobble would be "
              "compositor-side.")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 and sys.argv[1] != "-v" else "-")
