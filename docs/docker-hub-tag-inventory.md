# Docker Hub ARMOR tag inventory

Snapshot completed at `2026-08-21T21:24:09Z` for the private
`ronaldraygun/armor` repository.

The Docker Registry v2 `tags/list?n=1000` endpoint returned 70 tags in one
page (with no pagination link). Each tag was then resolved with an
authenticated `HEAD /v2/ronaldraygun/armor/manifests/<tag>` request. The
`Docker-Content-Digest` header is the canonical manifest or image-index digest
for the tag, not a platform-specific config or layer digest.

There are 67 `0.1.*` release tags, two short Git-SHA tags, and `latest`.
Manifests are `application/vnd.docker.distribution.manifest.v2+json` unless
noted as an OCI image index.

| Tag | Manifest digest | Media type |
| --- | --- | --- |
| `0.1.0` | `sha256:4f25977f34c4d56bd0f4a471b306b3c6d8f1a52f479b40d34c93529bc7580e6f` | Docker v2 manifest |
| `0.1.1` | `sha256:cce164183f2b1d86534c169ffa94f9385200bc6a798ec8dc23a4984c74e4eaa1` | Docker v2 manifest |
| `0.1.2` | `sha256:085daaa561957df885af4954674dbdd525c6182747250694b66b0018c5e1fa16` | Docker v2 manifest |
| `0.1.3` | `sha256:8524d22c345944630d7bcb0f1fe3de61fb098091a62fdcfe8056a092f2a9d70c` | Docker v2 manifest |
| `0.1.4` | `sha256:b7ad417ee27335157a8f2601862da5ffe48a1d35fd7f2a0187d729742d635111` | Docker v2 manifest |
| `0.1.5` | `sha256:26f904cf609747b26c95dc3b5efc3d6593fe9738484276fc5afb4a95e86b1f83` | Docker v2 manifest |
| `0.1.6` | `sha256:703a9247c83776f4e2109c4e2307cff0be6cccf4ae2cc9ff329c90ba6e99ba78` | Docker v2 manifest |
| `0.1.7` | `sha256:cce61309d356a6dc12e6e717429565cf687f6e3f9b035beff21fe9cb14b1e36c` | Docker v2 manifest |
| `0.1.8` | `sha256:691d9ffea0829beffa96349a22c8a4cc8d6c9e57789b7dda6971b1cf41bf699d` | Docker v2 manifest |
| `0.1.9` | `sha256:6de49b6d8c6db8275858e7b0a48b2162c634386e84ed13cae1f1429d4d7b2396` | Docker v2 manifest |
| `0.1.10` | `sha256:9a25af91186dd41aa8bd77f6fcad37d5b7b051426a8e96a82100722e6fc09a2c` | Docker v2 manifest |
| `0.1.11` | `sha256:f45383a6251f6c141664e224b7110d6135b5a21c7b55f3c9bf56cbebf90b50bc` | Docker v2 manifest |
| `0.1.12` | `sha256:4b09f6e118992731b7e2a356e323fda0230fe4d421420009ef58caf8291cbc41` | Docker v2 manifest |
| `0.1.13` | `sha256:bf96281f176cf0793fdaad6a1a9b344eb70e0064042f8f92804402c2437d6cea` | Docker v2 manifest |
| `0.1.14` | `sha256:d162f2be9cea137396dce83caee2f45d2604be2f1c34b8aaf49c729ab8e70ca0` | Docker v2 manifest |
| `0.1.16` | `sha256:54a5e51d9c7aa38208bbb197774cb731e19a7276ea319e01b742e7cffcc779c3` | Docker v2 manifest |
| `0.1.17` | `sha256:b9fe6ba9e01cfae24104ae373689c8303cfaf2cd77808ccf0b3f86777b23d1fb` | Docker v2 manifest |
| `0.1.18` | `sha256:8e9045acc2aaa67d39cc117b6f52e807f16280d28b1d1b7b40eccfd75f4d2026` | Docker v2 manifest |
| `0.1.19` | `sha256:52941e7c79f91ce40c64e7435307727585aba45302794cca62a6520a5f1c2208` | Docker v2 manifest |
| `0.1.20` | `sha256:994a347cea3a68d130991719d359eb03fc94444beb619fcc8e918d39c2f84934` | Docker v2 manifest |
| `0.1.21` | `sha256:6e298eb159c8c70bb1d7ca7bf7e809f3a85b8066ab763018bf7b6f8123dd152f` | Docker v2 manifest |
| `0.1.22` | `sha256:f2a70704be509b818e5d4f8b32c91cac5b27e54a57b91acc6505b9dd82ba3f15` | Docker v2 manifest |
| `0.1.23` | `sha256:f37465fd253fc1f83c3f20fc64b5fc0b99396698bb814fd06361e51dca570d4b` | Docker v2 manifest |
| `0.1.24` | `sha256:6ff0dc7e8aacc17b3c21fe4dc0c29ba889c95684807edf6cceaad127865348df` | Docker v2 manifest |
| `0.1.37` | `sha256:0cbb8306a89b1b7117a1848b0161aa2f7c103f1d9069543f35aa5f6642ce0ea1` | Docker v2 manifest |
| `0.1.38` | `sha256:316ff6564bdcb55fc91a501b9a0b18a6a0e3d54bdbc031eabfd0df48566ccf2b` | Docker v2 manifest |
| `0.1.39` | `sha256:b0ee5bfa2a92d7377cde3c7fb62db77ed2f145f6bc939b1a4276f99253da3adb` | Docker v2 manifest |
| `0.1.40` | `sha256:a019b1813bdf1c4f32c547a8cfb4a0d17909cf5cfb659032af163e5c66ca1ffb` | Docker v2 manifest |
| `0.1.42` | `sha256:25a701d32cbbc2e3176d4d34b89389c41d279fe0332a11bde266c95f473dd846` | Docker v2 manifest |
| `0.1.43` | `sha256:db53e47dda88548ee48d7f74e7612d3a534bfdfedc32ef313c821aafc5076059` | Docker v2 manifest |
| `0.1.1833` | `sha256:6e686e389e3f6a03ae9b18e167727a15e00df2d97448b29a64c1b43f04ead53c` | Docker v2 manifest |
| `0.1.1834` | `sha256:dec262f027353af959335f1594caae392eee5e01184f3aa5614fd0d67b17481f` | Docker v2 manifest |
| `0.1.1835` | `sha256:b3c4fe820067c63c5520b7e49eba98ce333e9df6f490d73b1ad9ccd638b72942` | Docker v2 manifest |
| `0.1.1836` | `sha256:5be7b3b639207d78fb07bcfa990cc12d0d11ab7d973b4ae0c1c39581190d2cbd` | Docker v2 manifest |
| `0.1.1837` | `sha256:b2d1fff61c7248bf02108063e00932feb15076aee9bab3438e808cf6b8f45da8` | Docker v2 manifest |
| `0.1.1870` | `sha256:5685891f269d6567fdf5fefd998c7753d26f053222197020d8869a4d0164825b` | Docker v2 manifest |
| `0.1.1871` | `sha256:ce327bb050fe03d320f91dd29c281f110f0dedfdb156803470ba8fe1aa014de1` | Docker v2 manifest |
| `0.1.1873` | `sha256:9d65a63525aac9cacaad17ee5e07ea222909c95a3a0fbf264c5fc6d1fc6fa826` | Docker v2 manifest |
| `0.1.1874` | `sha256:1344c7e550331c5f344becd88d70201aa17d817218074169513ae4292c6e641d` | Docker v2 manifest |
| `0.1.1876` | `sha256:e2e06c08ac79eb87b78c3748b9e8344ec4c0f4bc6a09b8d2cb6ed49755d14ef9` | Docker v2 manifest |
| `0.1.1878` | `sha256:940e4c71926decb0a8362de9ee16cdbc6499a578c8b38c582b1fde1803bbdd55` | Docker v2 manifest |
| `0.1.1880` | `sha256:664ff307d4c053680ef2d69eb95ea461064391be2088a101fce24c902a2d6211` | Docker v2 manifest |
| `0.1.1881` | `sha256:8f365dec3b0388bcfe00b43474f44c1e139d493d0e9dae6a12ac63e9e6ca8610` | Docker v2 manifest |
| `0.1.1882` | `sha256:ba910bf4f6dd5a79e6de46e2ee322378ea2b5c7a7b2f6876f80e574cea1fefdb` | Docker v2 manifest |
| `0.1.1887` | `sha256:1471c16ad85cedbda9a99d003b9529f583ae230a7c3b722a4d91d891a4968e15` | Docker v2 manifest |
| `0.1.1893` | `sha256:116085fa9a2bab244de4fe3d478af5978a8584554a42c1d4af53a30abeb4a7b9` | Docker v2 manifest |
| `0.1.1896` | `sha256:75c157c9e487a3584362f7ff4e68bfbc4dafd4110923e0b13974d06ec2615436` | Docker v2 manifest |
| `0.1.1900` | `sha256:cb9fea1c27a8319ea135003509ef490fdba3e96c27f66f4c4b05775f9092125c` | Docker v2 manifest |
| `0.1.1901` | `sha256:8eb828e0b0f9bd7a1ab452b605463dd51a669550f8152fcc2230743ba96176e5` | Docker v2 manifest |
| `0.1.1902` | `sha256:73dd1cff2d147e23a7c39fb8b71e9b6d896000fced8ab4fb8617edcde745dc8f` | Docker v2 manifest |
| `0.1.1904` | `sha256:9b0be1fe11585999460ac5b963adb22688a1d89a6e06ddc06b56c60262d259c2` | Docker v2 manifest |
| `0.1.1905` | `sha256:14a2f670c566a08cbaa444b3f1fefffd7521e175e30fda7dc4bf7da94361b6a2` | Docker v2 manifest |
| `0.1.1906` | `sha256:6120ac8b0c050175b25bcd53dccc8d21a1973a65dfb5eea104e94f494716beb2` | Docker v2 manifest |
| `0.1.1908` | `sha256:7f7beaff114e5430694476e3da7c36ffc1c7dcea3386e3dda0290f4a5e75d3f1` | Docker v2 manifest |
| `0.1.1909` | `sha256:ead8458b649b1d2a94ab74bf7a0dfc2ff854d5e98e7cbba8e27bfb6779f1e674` | Docker v2 manifest |
| `0.1.1910` | `sha256:116f02506bcc41fd6151054cb0e8490b769920ea65bbafddcfa458356487ea05` | Docker v2 manifest |
| `0.1.1911` | `sha256:b93e7e0b58c6b23cc041f1198adf301c7e2351330cdeaef09a7c106fd42a5b07` | Docker v2 manifest |
| `0.1.1912` | `sha256:c86fe808c71e434a63b1c9202fad570ca22ae99717f472afd0096de73b101eec` | Docker v2 manifest |
| `0.1.1913` | `sha256:e5ceb005c873ee8769af9bdcaec3d15fbe7e6704addf74a8de2870f36dd96ca0` | Docker v2 manifest |
| `0.1.1914` | `sha256:32343367cb415d1f8cca52c52861689f6a1fb8f9204c189a1a31a77c71a71a2c` | Docker v2 manifest |
| `0.1.1915` | `sha256:ef7ff098467ee3b6404c3957d68fa13fabe19c1b007a5ea687cfe4da896afc7b` | Docker v2 manifest |
| `0.1.1916` | `sha256:5ce37f4f452b19ff9346a3abfa70cc19fa7d08343315e0ddb86f2f8bd7b668e4` | Docker v2 manifest |
| `0.1.1918` | `sha256:e12b558796eb453d1ec183c57b7293c9c4dc0d148a1658270f2bfa725d6bf601` | Docker v2 manifest |
| `0.1.1919` | `sha256:e3a6b506954f642e0b00a35e36f657008f97674f65ec239643cd26709e8eebfc` | Docker v2 manifest |
| `0.1.1920` | `sha256:ec29dca5287d47c22cbb7d84f242da6c28c9d3b6c38c9b1674fc5c26fb5b4b5f` | Docker v2 manifest |
| `0.1.1922` | `sha256:a06a750f96bf1efa399a8542962c1788f0252b0d8599eb1be0b739210ddbf002` | Docker v2 manifest |
| `0.1.1923` | `sha256:c5d6e68d11f8b601709b7d4850d8158f7981d56e41ca3215e4c90210c30732d0` | OCI image index |
| `5f8dd6f` | `sha256:6bc15fa346af12b0853011bd99505c401832c52d578c427595db703ae2d9e02a` | OCI image index |
| `fcbf6d3` | `sha256:acbf5a1afc029d0d6a3b918b62e453c8b5801e1301ba2c4e105af399dac2916e` | OCI image index |
| `latest` | `sha256:7182d0d73649728a7fdf3571b010f0e07d964f05e95f775290a562869a1ed38b` | Docker v2 manifest |
