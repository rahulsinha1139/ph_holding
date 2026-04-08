# Master Data Cleanup Review

Generated: 2026-04-08T15:06:47+05:30

## Customers

### AL-FARIS Industrial Supplies W.L.L

- Normalized key: `AL FARIS INDUSTRIAL SUPPLIES`
- Auto-merge: `true`
- Reason: secondary customer records have no operational history
- AL-FARIS Industrial Supplies W.L.L (`8df3d129-d7b6-4369-963b-fcf3d4ab6568`) invoices=0 orders=0 offers=0 contacts=1 opportunities=0 score=104
- Al Faris Industrial supplies (`164aba64-8341-4774-b89d-453ebd918d85`) invoices=0 orders=0 offers=0 contacts=0 opportunities=0 score=68

### ALUMINIUM BAHRAIN B.S.C

- Normalized key: `ALUMINIUM BAHRAIN`
- Auto-merge: `true`
- Reason: secondary customer records have no operational history
- ALUMINIUM BAHRAIN B.S.C (`13488c9f-e5e7-4c46-a106-e7364d38b08d`) invoices=23 orders=13 offers=4 contacts=25 opportunities=59 score=32543
- ALUMINIUM BAHRAIN B.S.C.(C) (`dce5b945-7df8-45db-b8eb-d5d7f752ca62`) invoices=0 orders=0 offers=0 contacts=0 opportunities=0 score=72

### ALUSERV MIDDLE EAST

- Normalized key: `ALUSERV MIDDLE EAST`
- Auto-merge: `true`
- Reason: secondary customer records have no operational history
- ALUSERV MIDDLE EAST (`53b3f79d-c94f-4a73-a523-2e27b1922a42`) invoices=3 orders=1 offers=1 contacts=1 opportunities=4 score=3804
- AluServ Middle East W.L.L (`b1e502d3-24be-4286-be1a-176dac7fbdd6`) invoices=0 orders=0 offers=0 contacts=0 opportunities=0 score=80

### BAHRAIN SUGAR REFINERY W.L.L

- Normalized key: `BAHRAIN SUGAR REFINERY`
- Auto-merge: `true`
- Reason: secondary customer records have no operational history
- BAHRAIN SUGAR REFINERY W.L.L (`fca4d293-fdc4-4040-a063-9e14244bec52`) invoices=22 orders=9 offers=0 contacts=3 opportunities=10 score=25628
- Bahrain Sugar Refinery (`b0405d38-5b92-47a1-8ef5-5c53db9bc944`) invoices=0 orders=0 offers=0 contacts=0 opportunities=0 score=52

### DELTA ELECTRICAL ITEMS WLL

- Normalized key: `DELTA ELECTRICAL ITEMS`
- Auto-merge: `true`
- Reason: secondary customer records have no operational history
- DELTA ELECTRICAL ITEMS WLL (`d2353d1a-b22e-4c3c-b42a-3bf89fd7a112`) invoices=0 orders=0 offers=0 contacts=0 opportunities=0 score=81
- Delta electrical items (`a2e632a0-918b-49dc-9ad7-e1afddce7203`) invoices=0 orders=0 offers=0 contacts=0 opportunities=0 score=52

### KRAUM AND SPRUK TECHNOLOGIES W.L.L

- Normalized key: `KRAUM AND SPRUK TECHNOLOGIES`
- Auto-merge: `true`
- Reason: secondary customer records have no operational history
- KRAUM AND SPRUK TECHNOLOGIES W.L.L (`3ebe52b4-bccf-4ba4-b71f-9a03303466a4`) invoices=2 orders=0 offers=0 contacts=1 opportunities=4 score=2434
- Kraum & Spruk Technologies (`b19f9bf1-3b3e-4e82-a75f-982eeeb545f5`) invoices=0 orders=0 offers=0 contacts=0 opportunities=0 score=66

### Prudent Solutions W.L.L

- Normalized key: `PRUDENT SOLUTIONS`
- Auto-merge: `true`
- Reason: secondary customer records have no operational history
- Prudent Solutions W.L.L (`df6ead45-2227-4c7a-9b03-c0961e5ebefc`) invoices=0 orders=0 offers=1 contacts=1 opportunities=1 score=283
- Prudent Solutions (`1e9fef5e-d00f-4be9-bb16-e60abf9e9440`) invoices=0 orders=0 offers=0 contacts=0 opportunities=0 score=37

### TTSJV W.L.L

- Normalized key: `TTSJV`
- Auto-merge: `true`
- Reason: secondary customer records have no operational history
- TTSJV W.L.L (`eb00c440-65ac-4f06-a6ab-87a4c9f9d625`) invoices=35 orders=16 offers=3 contacts=3 opportunities=15 score=41451
- TTSJV (`f89ada4f-0b95-4c64-aa05-6d3b73e4cb0d`) invoices=0 orders=0 offers=0 contacts=0 opportunities=0 score=15

### Veolia Energy Gulf WLL

- Normalized key: `VEOLIA ENERGY GULF`
- Auto-merge: `true`
- Reason: secondary customer records have no operational history
- Veolia Energy Gulf WLL (`b1993ec0-6437-4a41-9d9b-e6e468dc6eb8`) invoices=0 orders=5 offers=4 contacts=0 opportunities=6 score=2537
- Veolia Energy Gulf W.L.L. (`afca44b9-a311-4c41-905f-c7fdd9cbac44`) invoices=0 orders=0 offers=0 contacts=0 opportunities=0 score=80

### ZAYIKA TECHNICAL SERVICES W.L.L

- Normalized key: `ZAYIKA TECHNICAL SERVICES`
- Auto-merge: `true`
- Reason: secondary customer records have no operational history
- ZAYIKA TECHNICAL SERVICES W.L.L (`ee1d4267-828e-4fb4-ae54-eaa0e076d6d6`) invoices=4 orders=0 offers=0 contacts=1 opportunities=0 score=4101
- Zayika Technical Services (`0cd68814-818e-4d70-8f2c-2fa55c1cbef0`) invoices=0 orders=0 offers=0 contacts=0 opportunities=0 score=55

## Suppliers

### Endress+Hauser

- Normalized key: `ENDRESS AND HAUSER`
- Auto-merge: `false`
- Reason: review required because duplicate suppliers still carry live activity
- Endress+Hauser (`cdd8b428-f81a-4619-8c5d-bdda73b1a4d9`) pos=0 supplier_invoices=76 supplier_payments=554 contacts=0 products=0 score=330224
- Endress + Hauser (`1687c35c460eab09d6318179042e44a5`) pos=29 supplier_invoices=112 supplier_payments=0 contacts=0 products=0 score=107446

