GO_INST := my/ev/...
GO_DEPS := b go-flags struc gopacket yaml errs

errs-url := https://github.com/ikravets/errs
errs-dir := $(errs-url:https://%=%)
errs-cid := master

b-url := https://github.com/cznic/b
b-dir := $(b-url:https://%=%)
b-cid := master

go-flags-url := https://github.com/jessevdk/go-flags
go-flags-dir := $(go-flags-url:https://%=%)
go-flags-cid := master

struc-url := https://github.com/lunixbochs/struc
struc-dir := $(struc-url:https://%=%)
struc-cid := master

gopacket-url := https://github.com/google/gopacket
gopacket-dir := $(gopacket-url:https://%=%)
gopacket-cid := master

yaml-url := https://github.com/go-yaml/yaml
yaml-dir := $(yaml-url:https://%=%)
yaml-cid := v2

-include go.mk
-include local.mk
