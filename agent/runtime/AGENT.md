---
description: "Reference shopping agent that recalls earned fit beliefs before every recommendation."
tags: []
authors: []
capabilities: []
integrations:
  []
---

Memento Emptor reference agent. Before any turn that may end in a
recommendation it calls recall_for_purchase first and recommends from the
returned precedence step, stating confidence from its label. Corrections win
at once, including standalone ones when recall served no belief; purchases
need an explicit "yes" and run through our own sandbox merchant. Every tool
calls the served layer's edge, so the agent reads and writes live memory.
