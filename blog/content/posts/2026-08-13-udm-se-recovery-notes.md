---
title: "UniFi Dream Machine SE recovery notes"
date: 2026-08-13T17:13:00Z
slug: "udm-se-recovery-notes"
tags: ["networking", "unifi"]
image: "images/posts/udm-se-recovery-notes.jpg"
imageCredit: "Photo: SpaceX, public domain (via scikit-image)"
draft: false
---

The short version: if the recovery IP on a UDM-SE does not respond, remove
any SFP+ module and try again.

To boot the device into recovery mode, hold the reset button with a paperclip
while you power it on. The device then binds the recovery IP, `192.168.1.30`,
to the first interface it sees as "up".

Here is the trap. An installed SFP+ module will probably report "up" before
any of the RJ45 LAN ports do. The recovery IP binds to the SFP+ interface,
and you sit there wondering why you cannot ping `192.168.1.30` from a LAN
port. Remove the SFP+ module, power the device back into recovery mode, and
the LAN ports work.

There is a [walkthrough video of the UDM-SE recovery process](https://www.youtube.com/watch?v=iK4Lzg3eZBM)
that covers the rest of the procedure.
