# Round 2 freeze: the shoppers, answers, population, prompts and grader are locked

Nothing about the round-2 shoppers, hidden answers, population, prompts,
extraction or grader changes after this commit. T10.4 runs all four arms
against exactly the files hashed below; any mismatch fails the gate before a
turn is spent.

- Base commit (main at branch time): `c374c66`
- Frozen tree: the landing commit of branch `task/t103` carrying this file.
  File contents are pinned by the hashes below, not by the commit hash:
  re-hashing the tree must reproduce every row of section 1.
- Offline: this file, the fixtures and every check it names were written and
  verified with no keys, no network and no spend. No arm 3 or arm 4 run
  exists anywhere in the tree (section 8).
- The gate runs the check: `tools/check.sh` calls
  `bench/round2/freeze_check.sh`, which extracts section 1, pins the row
  count and verifies every file with `sha256sum -c`.

## 1. Frozen hashes (sha256)

Every path is relative to the repository root. The hidden answers file is
pinned by hash only: a hash reveals nothing about the content, but any later
change to the key is detectable.

```manifest
4512d8a1a83e5c7a8e35d43691f534591db8d23faf1451718e386ed7a5467adc  bench/round2/shoppers/shoppers.json
509d591a97fdc7d6e418cc774f14debeb54064f73c4afe9367e66c3d79c69d62  bench/round2/hidden/answers.json
e1b36d34d5b5a52d70ac092a890678c8c27a3c8d80343f39ecb8a0fae70b869c  bench/round2/population/population.json
cbba9ab947a3ffa05f70d962b8a219eb4c6bb171335f0688c7698e77bd166119  bench/round2/paypal/r2-corr-belief/get-order-1.json
b3d14e9f37d87e29bf4bd3277efc3aff000a56bbd0416c56f5efbab3500452e3  bench/round2/paypal/r2-corr-belief/get-order-2.json
1d7c3c22084cbefaf9c7c12954792be71c83037e3def379ae293623ccfa81af1  bench/round2/paypal/r2-corr-belief/get-order-3.json
90a261d0e3664f514a2af48330ac9484c12ddaf119a96a3e47d8f077105f5317  bench/round2/paypal/r2-corr-belief/get-order-4.json
057afc9a34da023c763a0f7738d442093e500424dd31a2e508dff883f8afbdaf  bench/round2/paypal/r2-corr-belief/get-order-5.json
42e3f7617fc466d52cff421707d2367441d075b5885ff214d10b1abf4b594f15  bench/round2/paypal/r2-corr-belief/get-refund-1.json
732907a650dd6d4f987b4f6524e0a394e665712376572d72f4ebd3d263f9f4d0  bench/round2/paypal/r2-corr-belief/get-refund-2.json
6bb4c06098dda11ba58581bb2eb6f0d26545ac1a869ff1858471f104c7a9e3a6  bench/round2/paypal/r2-corr-belief/get-refund-3.json
3df81f6a70939f463666111c50b08d8e09320531ce80d262480fe7723962f6e8  bench/round2/paypal/r2-corr-belief/get-refund-4.json
e8b9340eb668d8aac0b670ee1dcc31a3c51298506a56077252d8d0500b75f470  bench/round2/paypal/r2-corr-belief/list-transactions-succeeded.json
92a01b8c506af6e7af906ee651e9caae7a0db0c9182a3f667ad156b15541b0f0  bench/round2/paypal/r2-corr-standing/get-dispute-1.json
60302789c81d23956909ff52ec72c50508c978c7aab5199738a3d4bcf7b34c37  bench/round2/paypal/r2-corr-standing/get-order-1.json
ad48759b2c54e9019899d47bd5e7b42be4e74036aee35501e3b401a0da972685  bench/round2/paypal/r2-corr-standing/get-order-2.json
9e8e441a6615ab3805b1595358686bf2cb59b8029cdcc4de04fa339cb6de8c15  bench/round2/paypal/r2-corr-standing/list-transactions-pending.json
6a63b0d3e0c08cf662c1b598b0457031b4b3c8946719c3fbe52eba7d92ab3285  bench/round2/paypal/r2-corr-standing/list-transactions-succeeded.json
1003a56edb641aa8fe40d18d545ce8ae0adb0fdf51beea75bf1d00058704e03c  bench/round2/paypal/r2-crowd-plain/get-order-1.json
f220ae093a092d063d0033ddfa688df146874aca2379114618bc69d036c95a31  bench/round2/paypal/r2-crowd-plain/get-order-2.json
030a535e01b4c2812b9a45c610b09c4a15fd44acb9fad0fbba9d4f8fb5f61167  bench/round2/paypal/r2-crowd-plain/get-order-3.json
496fcb60ec9c025b48e55e90ad16388afebff4b872dc8cfb18916b5032a3312d  bench/round2/paypal/r2-crowd-plain/get-order-4.json
9f652bad6994b98ec4dddf6dff08c5288400c49f9e04c31b15c0c0e31508900d  bench/round2/paypal/r2-crowd-plain/get-order-5.json
b3c04d77bd7094d298b8ef1e4c5337f59970991d943eb236694fdfc42b3c0114  bench/round2/paypal/r2-crowd-plain/get-order-6.json
5b41685f508235ef78e1005d302f7d133680b7f7fe8186e939d98430b1cd2646  bench/round2/paypal/r2-crowd-plain/get-order-7.json
b46b72c55515a66e820e06ca00bebf6f6e4660d7cb363a094a85d92bbd277a07  bench/round2/paypal/r2-crowd-plain/get-refund-1.json
a6a780cb82d065637bd58fd2435aa032e2bfe7aaea79c301011c20e769218822  bench/round2/paypal/r2-crowd-plain/get-refund-2.json
f4ff87fe972d065af176309ec8fd0824e847b5039ec5bf68ff5043af336d903c  bench/round2/paypal/r2-crowd-plain/get-refund-3.json
a4d8022bb98e354db1ce22dc148c1f2a23455aebd2ff1260309e22238dfff470  bench/round2/paypal/r2-crowd-plain/get-refund-4.json
a1636137336fad71f2dd392ffcfa89b57207856a44506e0c8539e42d804ac225  bench/round2/paypal/r2-crowd-plain/get-refund-5.json
0845da1709bd89d4202216f34ef8e484fe349a6adfb135a663f11a19c31251e8  bench/round2/paypal/r2-crowd-plain/list-transactions-succeeded.json
7a45b9616917be5e8aa5f282b8e629f4a7ab02d5921de201071ad57bc95e9748  bench/round2/paypal/r2-crowd-rival/get-dispute-1.json
7e0e08cdb325d1fe8f9831a106ceff0ef82f8ec33d520a1b18507d209c5f1332  bench/round2/paypal/r2-crowd-rival/get-order-1.json
01f724a1457cb42a2885aa1ff90a47bfe0dd53dd13bdbca342f45332583f9aca  bench/round2/paypal/r2-crowd-rival/get-order-10.json
5761e38ae81e39e07738114bc9f50ef49ccae2a2417dc5a389122065b0dfd761  bench/round2/paypal/r2-crowd-rival/get-order-11.json
e0f3a3725e62dab04a24c5f341d21d58a68a78b75a4dc806362fce436bc82ee7  bench/round2/paypal/r2-crowd-rival/get-order-2.json
705faf69aca326387bf72034c2a34a0bc0d5bb3d68091b3f5b0950eb0cf5bd1d  bench/round2/paypal/r2-crowd-rival/get-order-3.json
1d43e1334784af6a8a1197a73c9514c738c73f75a91640cd27f5bc7dcd5962d1  bench/round2/paypal/r2-crowd-rival/get-order-4.json
77295d96f5f0c189196e40742c4d6cedaf2c0e56da11906175d83423014e7384  bench/round2/paypal/r2-crowd-rival/get-order-5.json
c0fe7acf755293a177942f38433639e837a033160b98fbc0185629ffe56df2f1  bench/round2/paypal/r2-crowd-rival/get-order-6.json
53bfa6db6c872bbc98e0fed0df6030014d3fd094fc886f21f7e641aabf45c388  bench/round2/paypal/r2-crowd-rival/get-order-7.json
5acf6c19d78dc621837a790be35e0138cfaa8a785aca9afa4fc483ded938af4a  bench/round2/paypal/r2-crowd-rival/get-order-8.json
6d0c3f28578d1d342b20ed5151a5b37aacc6617baaea16918e85b4c5c1ba0e35  bench/round2/paypal/r2-crowd-rival/get-order-9.json
9895fc3350ba684f0ef213114ded30c4d42170f4ddd27099342e1e9c22ed82e6  bench/round2/paypal/r2-crowd-rival/get-refund-1.json
1d57243748fb7750ef29392086bff16b20f229bb67abc137eb3dfff43d669a6f  bench/round2/paypal/r2-crowd-rival/get-refund-2.json
eccec1a3c17078ffb22a4ae75093ab4595d077bbae3d8a987d95469f977db46d  bench/round2/paypal/r2-crowd-rival/get-refund-3.json
8a02f18f88fdde3cb3b862761c22199ac5f0700864aa403570078a608ac3f0bf  bench/round2/paypal/r2-crowd-rival/get-refund-4.json
2105733b199ea079b3cf2e94f39569d9a6c9c766c39500d7b044f90836ef392f  bench/round2/paypal/r2-crowd-rival/get-refund-5.json
f2de7a0b2172b90147559cde0b12c79e05f52c0a2b4f8e864821f45f29e6a0e6  bench/round2/paypal/r2-crowd-rival/get-refund-6.json
3ccc1a8c3c6e9d358c716d7722e1a0b0cb638e826218ef53e7d6d737dc202c88  bench/round2/paypal/r2-crowd-rival/get-refund-7.json
2d5710d2ef6e2cdf526a8d12e5033daffe49f8c40f601e5a0af455e5c9ba1752  bench/round2/paypal/r2-crowd-rival/get-refund-8.json
a45adc0130732f9c75ada3d1f9256fe230d8918be291eb6e0f29cdd7369723b7  bench/round2/paypal/r2-crowd-rival/get-refund-9.json
2a17b221c58dab96992969cbca3e389ce007f6e2e28a384a7500af1817e0bc24  bench/round2/paypal/r2-crowd-rival/list-transactions-pending.json
1fd8e4fe40b0288885a78325adc257ea747080ebac5eede867f5f1fff7cae52a  bench/round2/paypal/r2-crowd-rival/list-transactions-succeeded.json
6ee4566b7652b564103c04d6ef38a10fabb55f417cf70121ab1b48dadada96de  bench/round2/paypal/r2-new-otherbrand/get-order-1.json
58c80da5d5ba1126cce617cc3cfe653c276f889f592c1cdba1e9b0118f8f647d  bench/round2/paypal/r2-new-otherbrand/get-order-2.json
9558a239328928c3c1222531831d379c96e5cd124928329bb88e77174ebfd6bc  bench/round2/paypal/r2-new-otherbrand/get-order-3.json
b5ebea596b7d8fa6d162e6c70918332cb48058516c309a7ea190a91aa847e19c  bench/round2/paypal/r2-new-otherbrand/get-refund-1.json
e0d89e0da10472de1660b55e2ee1ad489de2a0a329b5d3ff63373120f98c5b16  bench/round2/paypal/r2-new-otherbrand/get-refund-2.json
5aa8fbf4b30590aa7a27323864f10e73db21b566d1a1fe067acf4ca936988a65  bench/round2/paypal/r2-new-otherbrand/list-transactions-succeeded.json
d1dc20ce051ff169faa0ca0b3fb5448913525fb6f08070765f6d84bf91d88c87  bench/round2/paypal/r2-new-plain/get-order-1.json
438df3b04a38f476112f25be361904586f9f8891e319d9e4103f99c4801d9167  bench/round2/paypal/r2-new-plain/list-transactions-succeeded.json
4491ca47ab5566b0a142a9573b745100bd2939fe33387bfc2f42899287246c5b  bench/round2/paypal/r2-noisy-down/get-dispute-1.json
4a38074203838571bfc08c3738077acead242f8d9cf209c6d9b6327367c9206b  bench/round2/paypal/r2-noisy-down/get-order-1.json
acdaf5de4556e3e446986d9bd6962e4afce1908c7332f8ab2d1bd27710cb8d8d  bench/round2/paypal/r2-noisy-down/get-order-10.json
2e0f975ed6349013ba6cdd5b0960004fa0971fb8d1efbed109cc969694391d85  bench/round2/paypal/r2-noisy-down/get-order-2.json
b4a5b8cccd6ab10d07e06c8360906576cd1b667c82694ab9f8b74999e0d8c28b  bench/round2/paypal/r2-noisy-down/get-order-3.json
113a5a850009b7a4effb77c8976ce97265e03a994213a845c2ae7ad37bbd9ffa  bench/round2/paypal/r2-noisy-down/get-order-4.json
ba9451443d696cfd28857c9414430c02fdee889d5258104f2a952dfea939996d  bench/round2/paypal/r2-noisy-down/get-order-5.json
23479af43d324fe2c746ee117c098569f070cf04aafdc5b5cc8b427634af416c  bench/round2/paypal/r2-noisy-down/get-order-6.json
917e44cae53ef1764aa820f8498603c6d1887deb7cc0e3040662993d24ccc61c  bench/round2/paypal/r2-noisy-down/get-order-7.json
00155affd5ade69a039d99f70670026e0347399e250caab601952f55347fdb63  bench/round2/paypal/r2-noisy-down/get-order-8.json
19f41dfd3794111690eec9bbe6124bf7e8e5e33b6be2567dea849ca85fa8b8f5  bench/round2/paypal/r2-noisy-down/get-order-9.json
deb20c5effdcd36fd3c79f63f468a4452d4a50c05d342b5dd09287d9e19c4b02  bench/round2/paypal/r2-noisy-down/get-refund-1.json
d425a2ca5cbbbb241e86b36b40876ab21176ce5d797523384f112164b997efc7  bench/round2/paypal/r2-noisy-down/get-refund-2.json
f7f168b56ea106619b618d8f5cce7cfa3c3d49d78f50ea4d3c7612a738d15fd2  bench/round2/paypal/r2-noisy-down/get-refund-3.json
7717624df85354c337b09fa92ae07711e8c5b08de7a1901eb2f40683237b262a  bench/round2/paypal/r2-noisy-down/get-refund-4.json
bf34e1af811444174e29e3fca79237a66f6089219fef520056d504d00288337b  bench/round2/paypal/r2-noisy-down/get-refund-5.json
739fece8d2af6eca3c80f86dcd15523205eaa9f49f6cfff9ff7bb892922ddafb  bench/round2/paypal/r2-noisy-down/get-refund-6.json
13da05ae783c98a57e9f81b8d34f172309bb144744108a5cf20fc4c1a012561f  bench/round2/paypal/r2-noisy-down/get-refund-7.json
b36c36dc31e05b62f9c47330edc40319cb0fb57d86809298977100d3b28df845  bench/round2/paypal/r2-noisy-down/list-transactions-pending.json
bab0b8391c6b19de8209580f685801fda4e70bf015481c69d4705302da8bd6c5  bench/round2/paypal/r2-noisy-down/list-transactions-succeeded.json
83fbc9669e5bf0d93b278315c89ccfad73b2e0ba4cf30ff82aa3e6eb813492c4  bench/round2/paypal/r2-noisy-up/get-dispute-1.json
17cd9faa9859399b57e790b21472aa599528f47befe30aa6cfca21fb0d1a0114  bench/round2/paypal/r2-noisy-up/get-order-1.json
fbbcc6f44347f13f13b69abfd169faf3b9f0253dac2857936754e3b313f6ad3c  bench/round2/paypal/r2-noisy-up/get-order-10.json
acee62096f80c88b5cab158dd8621b69911e9226b6fd57f7c28b8cf20ed68869  bench/round2/paypal/r2-noisy-up/get-order-2.json
47daa0236620bac7d7d1c95f4c9f3bce7c0886e3c282150a6a5b7c69b4167459  bench/round2/paypal/r2-noisy-up/get-order-3.json
535879c851dd2418915084c1d2ea639c386ddcf78200a12ee236c4a8ba0f929d  bench/round2/paypal/r2-noisy-up/get-order-4.json
d7381f0df5ac1b608b9599fbb49a12616561cc622c9164e5394ae7953ab070d0  bench/round2/paypal/r2-noisy-up/get-order-5.json
369afc110bc25b1864b136b4df1dad98b142dbfc51a281f86609def5acc9887a  bench/round2/paypal/r2-noisy-up/get-order-6.json
38c902a8598414cc1a0ccbffce3e6ae114de4c2eeb4149c8daa4657d9a397a90  bench/round2/paypal/r2-noisy-up/get-order-7.json
7d732ca4ab861ecb458f0931491b9ebc7f9bdcedd5e17430fcfe699d94eb02a5  bench/round2/paypal/r2-noisy-up/get-order-8.json
334b158cec975c5202a87e11d68e14f84d0e3c7d481570ec3bf6b4d021b991c4  bench/round2/paypal/r2-noisy-up/get-order-9.json
cdc5be743988ee983b5cd8fd2c8fab01acb179d4ba5ebface34d415176d22e21  bench/round2/paypal/r2-noisy-up/get-refund-1.json
e936c73347294e877a280fd56236099bec2ad5b834cc3ccf55fe2d86cea65250  bench/round2/paypal/r2-noisy-up/get-refund-2.json
f52efeb5508b330a608d08cf0f933db933594b55fbdb9037de20f8075093c837  bench/round2/paypal/r2-noisy-up/get-refund-3.json
38bf4e7e7068a9d2a7de08d4d1dd650a5cc4c61a1022b90931c1ab60dc6c9e05  bench/round2/paypal/r2-noisy-up/get-refund-4.json
6277768f37bd447aff49ddeea12205f85d0c145d1517ab2c4996a7a3470f3498  bench/round2/paypal/r2-noisy-up/get-refund-5.json
0050f26f2344c97bfe9f980edc6ea317253927c0da0829e82b19587ee09cadf3  bench/round2/paypal/r2-noisy-up/get-refund-6.json
5d46d16fd70e5d3f0ce7a009af933e95d171c1d0da6537afec6987024f79a2a0  bench/round2/paypal/r2-noisy-up/get-refund-7.json
92b1c25b99bdd67fdb8a0ae8905cecdce3fcf490576b65fd0f48d758f92cea37  bench/round2/paypal/r2-noisy-up/list-transactions-pending.json
4d362ee7e6b8fe5522f5ea5c57670a90ee1cf2e1be23d299d540cd7b3ef98136  bench/round2/paypal/r2-noisy-up/list-transactions-succeeded.json
f90e5f9743accf1213b71d6a152e528ccdddb3c19de3618f2bcbabc5b34dcac4  seed/round2.go
20eeef1f433346af8170ff1fb60839e3d59a259b5013fd5b67d4c308c8a3f1d4  bench/round2/cases.go
2decee7b36d67e7eefe84151a72bb0fd58afec9e7d558b7f23b326b043b703a5  bench/round2/doc.go
33ad363b76718ea849bc1a5f15a800b9cff952f9a07b0877b68eb8195b4f43aa  bench/round2/facts.go
31b79b0ef1234bead9402cbf0791d74aaeffd5c19c2ff222938cdd796fd60428  bench/round2/grade.go
1b51fb0306353f499eabaecbe71816b3a02e9efa71ee6da01546406e3ed4822a  bench/round2/paypal.go
507be8782d82e39e65286e0b9185f97dfd118f8877fe6d9ecacae54ac814c5ee  bench/round2/prompt.go
ec8c3606faeac40055b834a241faa07d095841004a37fbc9000fae750d020431  bench/round2/reason.go
fabfedfe9f76da16009133bba06818d4d9230d8cf1d3f8907f9ea58b61831242  bench/round2/recall.go
7c1f7c408e4bf48d2d068b5ae8bb93b8fbfa318df7a2f778f9275f63eaca518d  bench/round2/runs.go
940bea8c758311e80b6a7071747eabef641f7ee29d95ce30a5b9a5fbfa30170d  bench/round2/arm2/extract.go
fbbf17d5bf0e8f23f2076e961de5744d350f55deecc104b0afda0ee5fa4d43ac  bench/pilot/pilot.go
2772f032e0b753981aa0f6696022ef1ef67b52af60d7a479e4cea3139d1b7c3d  bench/grader/grader.go
264900cfb53002d836109e9714c32c0910154e42daaec1307ae626f8ff06a926  bench/harness/harness.go
```

What each row is:

| Rows | Role in the comparison |
|---|---|
| `bench/round2/shoppers/shoppers.json` (1) | The arm-facing shoppers: eight histories, each with its case, its candidate line and its usual size. It carries no answer. |
| `bench/round2/hidden/answers.json` (1) | The key: one hidden answer per shopper, with the size right after a correction where there is one. No arm reads it; only the grader and the fairness check do. |
| `bench/round2/population/population.json` (1) | The seeded population behind the pool: every line and size the pool is read for, as `domain.FitRow`s with the seeded mark, the salted per-line shopper hash and the transaction hash. |
| `bench/round2/paypal/**` (99) | Arm 2's input: one directory per shopper of toolkit-shaped records (a transaction listing, `get_order` bodies with line items, `get_refund` bodies and `get_dispute` bodies), exactly what T10.1's extraction reads. |
| `seed/round2.go` | The round-2 generator (`Round2Seed` 41). Regenerating reproduces every fixture above byte for byte; a test proves it. |
| `bench/round2/cases.go` | The four cases with their separation statements, the arm numbers and the frozen hold table. |
| `bench/round2/facts.go`, `reason.go` | The facts each arm's input holds, and the reference reasoner the fairness check runs over them. |
| `bench/round2/prompt.go` | The prompt template: the base section every arm shares, the four memory blocks, and the project-recall rule. |
| `bench/round2/recall.go` | The expected `recall_for_purchase` response per shopper, built from the frozen history and population. |
| `bench/round2/paypal.go` | The arm-2 record generator, so the committed records can be rebuilt and compared. |
| `bench/round2/runs.go` | The run-directory protocol: the marker T10.4 writes to open `runs/`, and the check that binds it to this freeze. |
| `bench/round2/grade.go` | The round-2 run record and grader: four measures reuse round 1's rules, and the record adds the turn kind and the precedence step the memory served. |
| `bench/round2/arm2/extract.go` | T10.1's extraction, unchanged. It is the only thing that turns the toolkit's records into arm 2's prompt input. |
| `bench/pilot/pilot.go`, `bench/grader/grader.go`, `bench/harness/harness.go` | The reused frozen rules: the reply extraction (`ExtractSize`, `ExtractIDRefs`, the human override), the five round-1 measures, and the token counter and caps. Round 2 adds no second definition of any of them. |

Recompute every hash (offline, from the repository root):

```sh
sha256sum -c <(sed -n '/^```manifest$/,/^```$/p' bench/round2/FROZEN.md | grep '  ')
```

## 2. Scale, fixed before any run

- **Repeat count: two turns per shopper per arm.** Three was the task text's
  plan, and the budget does not hold it: the P9 run mean is $0.0337 per turn,
  so three repeats is 168 turns (96 first turns plus 72 correction turns) at
  about **$5.66**, over T10.4's $5.00 cap before any overrun. Two repeats is
  112 turns (64 first turns plus 48 correction turns) at about **$3.77**,
  which leaves room for a discarded turn or two. Two still separates a stable
  answer from a lucky one on eight constructed shoppers, and the pilot's n=1
  is the thin signal this improves on.
- **Shoppers per case: exactly two.** Eight shoppers in all, four cases. The
  pair in each case exists so a single idiosyncratic history cannot carry the
  case: each pair disagrees about what makes the answer hard (a brand with no
  history versus another brand's cut; the rival-birth threshold versus a plain
  first return; noise that buries fit versus noise in the other direction; a
  belief contradicted versus no belief to contradict). Fairness did not force a
  third, so none was added.
- **Turns per arm:** 8 shoppers x 4 arms x 2 repeats = 64 first turns. Case 4
  adds the correction, next-session and other-agent turns: 2 shoppers x 4 arms
  x 2 repeats x 3 turns = 48. **112 turns in all**, on the model the freeze
  pins (`claude-sonnet-4-6`, the same string in every arm). T10.4 records the
  actual spend against the $5.00 cap.

## 3. The four cases, and why each separates the arms

Each case's separation is stated here, before any run, and pinned by a test
(`TestTheCasesSeparateTheArms`) that runs the reference reasoner over each
arm's input.

| Case | Shoppers | Why it separates the arms |
|---|---|---|
| New to the brand | `r2-new-plain`, `r2-new-otherbrand` | Neither shopper has a row on the candidate line. Arms 1 and 2 hold nothing (and arm 2 has no record on that line at all); arm 3 holds only the general statement, so it answers the usual size M; only arm 4's pool (Earned small at M) holds the answer, one rung up to L. The second shopper carries another brand's large cut, so an input that maps sizes across brands answers down and is wrong. |
| Unlike the crowd | `r2-crowd-rival`, `r2-crowd-plain` | The pool is Earned small at M, so the pool step alone answers L and is wrong. Both shoppers' own fit evidence points large and outranks the pool: arms 3 and 4 hold S. Arm 2 holds nothing, because the too-large returns sit beside non-fit refunds at the same size and the toolkit returns no refund reason. The first shopper's answer depends on the rival-birth threshold (section 5). |
| Noisy refunds | `r2-noisy-up`, `r2-noisy-down` | Each history carries three fit returns of one direction among damage, non-arrival, changed-mind, vague and unexplained refunds at the same size, so arm 2's records cannot say which returns were about fit and it holds nothing. Arms 3 and 4 hold the answer from the reasons; their pool is Claimed, so the pool plays no part. One shopper sizes up and one down, so an input that only ever adjusts one way cannot pass. |
| Correction, then another agent | `r2-corr-belief`, `r2-corr-standing` | The correction travels through the layer, not through a conversation, so arms 3 and 4 hold the corrected size for an agent that never saw agent A's turn; arm 2 holds nothing because the correction appears in no PayPal field, and arm 1 holds nothing. The first shopper's belief is earned and stands contradicted beside the correction rather than deleted; the second has no belief, so the correction stands on its own and still decides. |

## 4. Fairness: a perfect reasoner on each arm's input

`bench/round2/reason.go` is the frozen reference reasoner: product.md's
precedence order over exactly the memory each arm carries, reading the frozen
history and population and never the hidden answer. `TestFairness` runs it for
every shopper and every arm and checks it against the frozen hold table in
`cases.go`, on the first recommendation and again after the correction. Where
the table says an input holds the answer, the reasoner must produce the hidden
answer; where it says the input does not, the reasoner must not produce it, and
must name the fact the input lacks.

The "Missing" column of the code table is not prose: each entry is one of the
`Missing*` constants in `reason.go`, the exact text the reasoner returns, and
`TestFairness` requires the two to match character for character. An entry is
also required wherever the table says an input holds no answer, and forbidden
where it says the input holds one, so the freeze cannot record an arm as
unanswerable without saying why, and cannot quietly answer it later. The table
the reasoner satisfies:

| Shopper | Answer | Arm 1 | Arm 2 | Arm 3 | Arm 4 |
|---|---|---|---|---|---|
| `r2-new-plain` | L | no answer | no answer, no record on the line | statement only: M, not the answer | pool: L |
| `r2-new-otherbrand` | L | no answer | no answer, no record on the line | statement only: M, not the answer | pool: L |
| `r2-crowd-rival` | S | no answer | no answer, no cause | own evidence: S | own evidence: S |
| `r2-crowd-plain` | S | no answer | no answer, no cause | own evidence: S | own evidence: S |
| `r2-noisy-up` | L | no answer | no answer, no cause | own evidence: L | own evidence: L |
| `r2-noisy-down` | S | no answer | no answer, no cause | own evidence: S | own evidence: S |
| `r2-corr-belief` | L, then M | no answer | no answer, no cause | own evidence L, then the correction M | own evidence L, then the correction M |
| `r2-corr-standing` | M, then S | no answer | no answer, no cause | statement M, then the correction S | statement M, then the correction S |

Two consequences are part of the freeze, not a surprise to report later:

- **Arm 3's answer in case 1 is a decoy, not an answer.** The shopper's
  general statement is memory, so the shopper-only arm always answers
  something; on a brand the shopper has never bought, that something is their
  usual size and it is wrong. The table therefore records "does not hold the
  answer", and the fairness test accepts a decided size there only when it
  differs from the hidden one. That is what makes the case a measurement of the
  pool rather than of the model's guessing.
- **No shopper's answer is derivable from PayPal's records alone.** A perfect
  reader of arm 2's input refuses on every one of the eight, and a test pins
  why: every fit return on a candidate line shares its line and size with a
  non-fit outcome, so the number and status of refunds cannot separate fit from
  damage, non-arrival, changed mind or an unexplained return. A fits-well
  verdict is exempt because PayPal shows it exactly as it shows a kept
  purchase, which says nothing about why anything came back.

## 5. The rival-birth threshold is mirrored, not approximated

The P2 review found round 1's drivers mirroring the direction rule but not
`domain.RivalBirthThreshold`: every opposite-direction row landed unattached
with no streak, while the driver claimed it landed below the threshold. Round 2
does not restate that rule in its own words. `bench/round2/reason.go` uses
`domain.OppositeStreak` and `domain.RivalBirthThreshold` from the layer's own
package, and `seed/round2_test.go` walks `r2-crowd-rival` through them to prove
exactly one rival is born.

The case is built so the difference is measurable rather than invisible:
`r2-crowd-rival` starts a runs-small belief with one return, then lands three
consecutive too-large returns (a changed-mind answer and a damaged return sit
between them and move no run, exactly as the service's answer path says). At
the threshold the layer births the opposite belief; two more too-large returns
back it, so it carries more evidence than the earlier belief and recall serves
it first. Without the threshold those repeats stay unattached, own evidence
still says L, and the cell grades the mirror instead of the layer: the case's
hidden answer is S, so a driver that omits the rule gets this shopper wrong.
The reference reasoner orders two beliefs by evidence and then recency, which
is why the rival is built with strictly more evidence than the belief it beats.

## 6. Arm 2's input: exactly the toolkit, no cause

- **Every record carries one custom id per shopper** (`r2-<shopper-id>`) in
  `custom_field`, the order's `purchase_units[].custom_id`, the refund's
  `custom_id` and the disputed transaction's `custom`. The listing has no
  shopper filter, so this is the only key the extraction can select on.
- **Every order gets its line items.** A record is written for every capture in
  the listing, so `get_order` is never the missing piece; a test proves the
  capture count and the order count agree for every shopper.
- **The seller's free text is fit-neutral.** Every transaction subject and
  refund note reads the same for every cause ("Store purchase, 1 item" /
  "Refund for the returned item"), and the item descriptions name the product
  and its size only. A test scans the extraction's output for cause vocabulary
  and fails on any hit, which is also what the reference reasoner refuses on.
- **The correction appears in no field**, so an agent holding PayPal's history
  alone cannot know it; a test asserts it.
- **Line items and order ids are supplied to every arm or to none.** No arm is
  given a PayPal order id it would have to cite: arm 1 has no memory, arms 3
  and 4 carry the layer's own belief and evidence ids (which are not PayPal
  ids), and only arm 2's own tool output carries the order and transaction ids
  it returned. Nothing is withheld from arm 2 that its tool would return, and
  nothing is handed to another arm that its own tool would not.

## 7. The fit-row key: one item per purchase, keyed by the capture

**No round-2 shopper has a multi-item order, and a test keeps it that way.** The
layer's fit row is one document per purchase, keyed by the hash of the
purchase's transaction id (the capture id, or the order id before a capture is
known). A second line item on one order would share that capture and therefore
that row, and whichever item's evidence landed last would overwrite the other's
line and size. That is a gap in the layer, not something round 2 may paper over:
a multi-item order needs a line-discriminated key (the transaction hash plus the
product line), which the writer does not mint. Round 2 therefore keeps one line
item per order, and the handoff log carries the issue text for the user to file
against the layer. The test asserts exactly one purchase unit and one line item
per order in arm 2's records, so a later shopper cannot quietly reintroduce the
case the key does not support.

## 8. The run directory: empty at the freeze, opened by a marker

No arm 3 or arm 4 run exists at this commit, and the tree enforces the order
rather than trusting it.

- **At the freeze:** `bench/round2/runs/` does not exist. `freeze_check.sh`
  fails if the directory appears without the flag, and
  `TestRunsBeginOnlyAfterTheFreeze` fails if it exists without the marker.
- **T10.4's first action** is to open the directory with the marker:

  Concretely, T10.4 writes `bench/round2/runs/FREEZE.json` before its first
  turn, with `round2.BeginRuns("bench/round2/runs", freezeCommit, note)` in its
  setup code, where `freezeCommit` is the full commit id from
  `git log -1 --format=%H -- bench/round2/FROZEN.md`. The same JSON written by
  hand works; the marker is:

  ```json
  {
    "freeze_commit": "<the full 40-character commit id that landed FROZEN.md>",
    "frozen_manifest_sha256": "<sha256 of bench/round2/FROZEN.md>",
    "note": "<who opened the runs and for which task>"
  }
  ```

  `BeginRuns` fills both values; `bench/round2/runs.go` is the one place that
  writes and reads it. `sha256sum bench/round2/FROZEN.md` prints the hash for a
  hand-written marker.
- **The marker binds the directory to this freeze, not to a later one.**
  `ReadRunsMarker` refuses a marker whose manifest hash differs from the file
  in this tree, so editing the freeze without opening a new run directory fails
  the test rather than silently measuring against a freeze that no longer
  exists. The commit id is checked for shape (forty lowercase hex characters)
  rather than against git history, because CI checks out a shallow clone and a
  history-dependent test would fail there; the manifest hash is the binding
  that holds everywhere.
- **The gate then allows the directory:** `tools/check.sh` passes
  `--allow-runs` to `freeze_check.sh` when, and only when, the marker exists.
  With the marker, hashes are still verified in full and the runs refusal is
  skipped, so T10.4 can commit runs with the gate green. Without it, the
  directory is a freeze break, which is what keeps a run from landing before
  the freeze.

Reproduce the pre-freeze state and both modes:

```sh
ls bench/round2/runs                                  # must fail at this commit
go test ./bench/round2/ -run TestRunsBeginOnlyAfterTheFreeze
bench/round2/freeze_check.sh                          # refuses a runs directory
bench/round2/freeze_check.sh --allow-runs             # verifies hashes, allows it
```

## 9. What the arms are given, and the one projection rule

- **Arm 1** carries no memory at all.
- **Arm 2** carries the T10.1 extraction of that shopper's toolkit records, and
  nothing the layer produced.
- **Arm 3** carries the layer's `recall_for_purchase` response for the shopper
  with the pooled product fit set to `null`: the shopper's own beliefs,
  evidence, labels and corrections, not the crowd's.
- **Arm 4** carries the same response with the pool block intact.
- **Both Memento arms** get the response with the `guidance` block dropped. That
  block is the layer's own recommendation in prose ("Size L up to..."), and
  leaving it in would hand both arms the layer's answer instead of the memory
  the answer rests on; the precedence objects, the beliefs, their evidence, the
  overriding correction and the pending count all stay. This is the only
  difference between the two Memento arms, and a test pins it.

The recall request each Memento arm sends is frozen too (`Target` in
`bench/round2/facts.go`): shopper, brand, category, fit handle and the usual
size. The pool read needs all four fields; a request missing the category or the
fit handle answers "unavailable", which would silently remove the pool from
cases 1 and 2, so a test refuses an empty one.

**A cell's turns run in two phases.** Case 4's first turn happens before the
shopper corrects, so the layer serves a response with no `overriding_correction`
and nothing contradicted; every later turn — the correction turn, the next
session and the other agent — happens after it, with the correction in force and
the belief contradicted beside it. `ExpectedRecall` takes the phase and
`TurnPhase` maps each turn kind to it, so a first turn can never be rendered from
a post-correction memory and answer before the shopper spoke.
`TestFirstTurnPrecedesTheCorrection` asserts the first-turn prompt carries no
correction text, the later-turn prompt does, and a shopper with no correction
gets byte-identical memory in both phases. T10.4 records the layer's live
response per phase, and `recall_for_purchase` on the first turn must be called
before `record_correction`.

## 10. Handoffs for T10.4 and the orchestrator

- **Seed the population with the moment of seeding, not the fixture's anchor.**
  Call `seed.GenerateRound2Population(time.Now().UTC())` (or the committed
  fixture through the same writer) and write it with the fit index's per-shopper
  writer, as `deploy/seed` does for round 1. The fixture's `AsOf` exists so the
  committed bytes are stable; reusing it later drifts rows out of the recent
  window and turns an Earned line Out of true.
- **The round-2 lines are all `regular`-handle on purpose.** The served layer
  resolves an item's line through `seed.PopulationLines()` and falls back to the
  regular handle for a line the round-1 population never seeded. A round-2 line
  with another handle would pool under a different line than the frozen pool
  names. Every round-2 line therefore uses `regular`, and the population file is
  the truth for its counts.
- **Seed each shopper's history into their own session** with the same shape the
  demo uses: a `derive` for the general statement, structural writes for the
  classified returns and the feedback, each citing the belief in `depends_on`
  and carrying its event time, so promotion can reach the standing the Memento
  arms need. The shopper's fit beliefs must reach at least Candidate to be
  served: every shopper whose answer rests on own evidence carries at least two
  fit returns in that direction, and three where the case's answer needs the
  belief to outrank the pool, spread over at least 0.3 of the session extent.
- **Record the live recall response** in the run file and compare it with
  `bench/round2/recall.go`'s expectation. Report a difference; do not repair it
  by editing a frozen input.
- **Multi-item orders are a layer issue** to file, per section 7: the fit row's
  key is the capture, so a two-line order would overwrite one line's row with
  the other's.

## 11. Deviations

- The freeze check is a new script (`bench/round2/freeze_check.sh`) inside this
  task's owns, wired into `tools/check.sh`. It pins 116 rows and
  fails on a tampered, missing or added row, exactly as round 1's check does.
- The gate's round-2 call is conditional: it adds `--allow-runs` only when the
  run marker exists (section 8). That is the one place this round's wiring
  differs from round 1's, which has no run directory to open.
- The repeat count is two, not the task text's three, because the budget does
  not hold three (section 2). T10.4's task text still says three; the freeze
  wins until the orchestrator reconciles it.
- No sandbox call was made for this task. The PayPal records are generated from
  the frozen histories in the shape T10.1 recorded; T10.1's recordings remain
  the evidence for that shape.
- `bench/round2/arm2/` is read, hashed and called, never edited.
