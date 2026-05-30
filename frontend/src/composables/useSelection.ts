import { ref, computed, type Ref } from 'vue'

/**
 * 列表多选 composable（C8）：集中「全选 / 半选 / 单项切换 / 清空」逻辑，
 * 消除 Configs.vue 与 Tasks.vue 中逐字重复的 selectedIds / isAllSelected / isIndeterminate /
 * toggle / toggleAll 实现。行为与原内联实现完全一致。
 *
 * @param items  受选列表（响应式 ref），全选/半选基于其当前长度计算
 * @param getId  从单项取出其数值 id（用作选中标识）
 */
export function useSelection<T>(items: Ref<T[]>, getId: (item: T) => number) {
  const selectedIds = ref<number[]>([])

  const isAllSelected = computed(() => items.value.length > 0 && selectedIds.value.length === items.value.length)
  const isIndeterminate = computed(() => selectedIds.value.length > 0 && selectedIds.value.length < items.value.length)

  /** 切换单项选中状态（已选则取消，未选则选中）。 */
  const toggle = (id: number) => {
    const index = selectedIds.value.indexOf(id)
    if (index === -1) selectedIds.value.push(id)
    else selectedIds.value.splice(index, 1)
  }

  /** 全选 / 取消全选：当前已全选则清空，否则选中当前列表全部项。 */
  const toggleAll = () => {
    if (isAllSelected.value) selectedIds.value = []
    else selectedIds.value = items.value.map(getId)
  }

  const clear = () => {
    selectedIds.value = []
  }

  return { selectedIds, isAllSelected, isIndeterminate, toggle, toggleAll, clear }
}
