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
returned guidance, citing the belief and its evidence. Corrections win at
once; purchases need an explicit "yes" and run through our own sandbox
merchant. The memory tools are fake-backed until the layer is served.
