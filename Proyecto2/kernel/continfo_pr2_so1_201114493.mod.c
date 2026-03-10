#include <linux/module.h>
#include <linux/export-internal.h>
#include <linux/compiler.h>

MODULE_INFO(name, KBUILD_MODNAME);

__visible struct module __this_module
__section(".gnu.linkonce.this_module") = {
	.name = KBUILD_MODNAME,
	.init = init_module,
#ifdef CONFIG_MODULE_UNLOAD
	.exit = cleanup_module,
#endif
	.arch = MODULE_ARCH_INIT,
};



static const struct modversion_info ____versions[]
__used __section("__versions") = {
	{ 0x466ebb20, "single_open" },
	{ 0x40c7247c, "si_meminfo" },
	{ 0xc2fb2181, "seq_printf" },
	{ 0x8d522714, "__rcu_read_lock" },
	{ 0x7113f676, "init_task" },
	{ 0x2469810f, "__rcu_read_unlock" },
	{ 0xf0fdf6cb, "__stack_chk_fail" },
	{ 0x8977aa3d, "proc_remove" },
	{ 0x73493e5e, "seq_read" },
	{ 0xd06ac449, "seq_lseek" },
	{ 0x1f9c313d, "single_release" },
	{ 0xbdfb6dbb, "__fentry__" },
	{ 0xda5bc0ce, "proc_create" },
	{ 0x122c3a7e, "_printk" },
	{ 0x5b8239ca, "__x86_return_thunk" },
	{ 0x14a1eb25, "module_layout" },
};

MODULE_INFO(depends, "");


MODULE_INFO(srcversion, "AFA006ECD097F56E887138F");
