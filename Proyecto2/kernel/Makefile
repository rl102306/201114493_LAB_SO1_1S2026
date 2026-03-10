obj-m += continfo_pr2_so1_201114493.o

KERNEL_DIR := /lib/modules/$(shell uname -r)/build
PWD        := $(shell pwd)

all:
	$(MAKE) -C $(KERNEL_DIR) M=$(PWD) modules

clean:
	$(MAKE) -C $(KERNEL_DIR) M=$(PWD) clean

load:
	sudo insmod continfo_pr2_so1_201114493.ko
	dmesg | tail -5

unload:
	sudo rmmod continfo_pr2_so1_201114493
	dmesg | tail -5

.PHONY: all clean load unload
