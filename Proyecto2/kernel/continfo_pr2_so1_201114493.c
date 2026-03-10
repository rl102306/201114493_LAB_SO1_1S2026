#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/init.h>
#include <linux/proc_fs.h>
#include <linux/seq_file.h>
#include <linux/sched.h>
#include <linux/sched/signal.h>
#include <linux/mm.h>
#include <linux/sysinfo.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("VGOMEZ - 201114493");
MODULE_DESCRIPTION("Modulo de kernel para telemetria de contenedores - Proyecto 2 SO1");
MODULE_VERSION("1.0");

#define PROC_FILENAME "continfo_pr2_so1_201114493"

static struct proc_dir_entry *proc_entry;

static int continfo_show(struct seq_file *m, void *v)
{
    struct sysinfo si;
    struct task_struct *task;
    unsigned long total_mb, free_mb, used_mb, total_kb;
    unsigned long vsz_kb, rss_kb, mem_percent, cpu_percent;
    int first = 1;

    si_meminfo(&si);
    total_mb = (si.totalram * si.mem_unit) >> 20;
    free_mb  = (si.freeram  * si.mem_unit) >> 20;
    used_mb  = total_mb - free_mb;
    total_kb = (si.totalram * si.mem_unit) >> 10;

    seq_printf(m, "{\n");
    seq_printf(m, "  \"memoria\": {\n");
    seq_printf(m, "    \"total_mb\": %lu,\n", total_mb);
    seq_printf(m, "    \"libre_mb\": %lu,\n", free_mb);
    seq_printf(m, "    \"usada_mb\": %lu\n", used_mb);
    seq_printf(m, "  },\n");
    seq_printf(m, "  \"procesos\": [\n");

    rcu_read_lock();
    for_each_process(task) {
        struct mm_struct *mm = task->mm;

        if (!mm) {
            vsz_kb = 0;
            rss_kb = 0;
            mem_percent = 0;
        } else {
            vsz_kb = (mm->total_vm * PAGE_SIZE) >> 10;
            rss_kb = (get_mm_rss(mm) * PAGE_SIZE) >> 10;
            mem_percent = total_kb > 0 ? (rss_kb * 100UL) / total_kb : 0;
        }

        cpu_percent = (task->utime + task->stime) / 1000000UL;

        if (!first)
            seq_printf(m, ",\n");
        first = 0;

        seq_printf(m, "    {\n");
        seq_printf(m, "      \"pid\": %d,\n", task->pid);
        seq_printf(m, "      \"nombre\": \"%s\",\n", task->comm);
        seq_printf(m, "      \"vsz_kb\": %lu,\n", vsz_kb);
        seq_printf(m, "      \"rss_kb\": %lu,\n", rss_kb);
        seq_printf(m, "      \"mem_percent\": %lu,\n", mem_percent);
        seq_printf(m, "      \"cpu_percent\": %lu\n", cpu_percent);
        seq_printf(m, "    }");
    }
    rcu_read_unlock();

    seq_printf(m, "\n  ]\n}\n");
    return 0;
}

static int continfo_open(struct inode *inode, struct file *file)
{
    return single_open(file, continfo_show, NULL);
}

static const struct proc_ops continfo_fops = {
    .proc_open    = continfo_open,
    .proc_read    = seq_read,
    .proc_lseek   = seq_lseek,
    .proc_release = single_release,
};

static int __init continfo_init(void)
{
    proc_entry = proc_create(PROC_FILENAME, 0444, NULL, &continfo_fops);
    if (!proc_entry) {
        printk(KERN_ERR "VGOMEZ [201114493]: Error al crear /proc/%s\n", PROC_FILENAME);
        return -ENOMEM;
    }
    printk(KERN_INFO "VGOMEZ [201114493]: Modulo cargado. /proc/%s creado.\n", PROC_FILENAME);
    return 0;
}

static void __exit continfo_exit(void)
{
    proc_remove(proc_entry);
    printk(KERN_INFO "VGOMEZ [201114493]: Modulo descargado. /proc/%s eliminado.\n", PROC_FILENAME);
}

module_init(continfo_init);
module_exit(continfo_exit);
