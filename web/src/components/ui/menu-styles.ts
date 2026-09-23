// Classes compartilhadas por DropdownMenu, ContextMenu e Menubar, para que os
// três tipos de menu tenham exatamente a mesma aparência (como no StarUML).
export const menuContent =
  'z-50 min-w-[11rem] overflow-hidden rounded-md border bg-popover p-1 text-popover-foreground shadow-lg ' +
  'data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 ' +
  'data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95'

export const menuItem =
  "relative flex cursor-default select-none items-center gap-2 rounded-sm px-2 py-1 text-[12.5px] outline-none " +
  "focus:bg-accent focus:text-accent-foreground data-[disabled]:pointer-events-none data-[disabled]:opacity-50 " +
  "data-[variant=destructive]:text-destructive data-[variant=destructive]:focus:bg-destructive/10 " +
  "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-3.5 [&_svg:not([class*='text-'])]:text-muted-foreground"

export const menuLabel = 'px-2 py-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground'
export const menuSeparator = '-mx-1 my-1 h-px bg-border'
export const menuShortcut = 'ml-auto pl-4 text-[11px] tracking-wide text-muted-foreground'
