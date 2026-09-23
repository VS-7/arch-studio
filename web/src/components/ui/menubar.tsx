import * as MenubarPrimitive from '@radix-ui/react-menubar'
import { CheckIcon, ChevronRightIcon } from 'lucide-react'
import * as React from 'react'
import { cn } from '@/lib/utils'
import { menuContent, menuItem, menuLabel, menuSeparator, menuShortcut } from './menu-styles'

function Menubar({ className, ...props }: React.ComponentProps<typeof MenubarPrimitive.Root>) {
  return <MenubarPrimitive.Root data-slot="menubar" className={cn('flex h-7 items-center gap-0.5', className)} {...props} />
}
const MenubarMenu = MenubarPrimitive.Menu
const MenubarSub = MenubarPrimitive.Sub

function MenubarTrigger({ className, ...props }: React.ComponentProps<typeof MenubarPrimitive.Trigger>) {
  return (
    <MenubarPrimitive.Trigger
      data-slot="menubar-trigger"
      className={cn('flex select-none items-center rounded-sm px-2 py-0.5 text-[12.5px] outline-none focus:bg-accent focus:text-accent-foreground data-[state=open]:bg-accent data-[state=open]:text-accent-foreground', className)}
      {...props}
    />
  )
}
function MenubarContent({ className, align = 'start', alignOffset = -4, sideOffset = 6, ...props }: React.ComponentProps<typeof MenubarPrimitive.Content>) {
  return (
    <MenubarPrimitive.Portal>
      <MenubarPrimitive.Content data-slot="menubar-content" align={align} alignOffset={alignOffset} sideOffset={sideOffset} className={cn(menuContent, 'min-w-[13rem]', className)} {...props} />
    </MenubarPrimitive.Portal>
  )
}
function MenubarItem({
  className, variant, ...props
}: React.ComponentProps<typeof MenubarPrimitive.Item> & { variant?: 'default' | 'destructive' }) {
  return <MenubarPrimitive.Item data-slot="menubar-item" data-variant={variant} className={cn(menuItem, className)} {...props} />
}
function MenubarCheckboxItem({ className, children, checked, ...props }: React.ComponentProps<typeof MenubarPrimitive.CheckboxItem>) {
  return (
    <MenubarPrimitive.CheckboxItem data-slot="menubar-checkbox-item" className={cn(menuItem, 'pl-7', className)} checked={checked} {...props}>
      <span className="absolute left-2 flex size-3.5 items-center justify-center">
        <MenubarPrimitive.ItemIndicator><CheckIcon className="size-3.5" /></MenubarPrimitive.ItemIndicator>
      </span>
      {children}
    </MenubarPrimitive.CheckboxItem>
  )
}
function MenubarLabel({ className, ...props }: React.ComponentProps<typeof MenubarPrimitive.Label>) {
  return <MenubarPrimitive.Label data-slot="menubar-label" className={cn(menuLabel, className)} {...props} />
}
function MenubarSeparator({ className, ...props }: React.ComponentProps<typeof MenubarPrimitive.Separator>) {
  return <MenubarPrimitive.Separator data-slot="menubar-separator" className={cn(menuSeparator, className)} {...props} />
}
function MenubarShortcut({ className, ...props }: React.ComponentProps<'span'>) {
  return <span data-slot="menubar-shortcut" className={cn(menuShortcut, className)} {...props} />
}
function MenubarSubTrigger({ className, children, ...props }: React.ComponentProps<typeof MenubarPrimitive.SubTrigger>) {
  return (
    <MenubarPrimitive.SubTrigger data-slot="menubar-sub-trigger" className={cn(menuItem, 'data-[state=open]:bg-accent', className)} {...props}>
      {children}
      <ChevronRightIcon className="ml-auto" />
    </MenubarPrimitive.SubTrigger>
  )
}
function MenubarSubContent({ className, ...props }: React.ComponentProps<typeof MenubarPrimitive.SubContent>) {
  return (
    <MenubarPrimitive.Portal>
      <MenubarPrimitive.SubContent data-slot="menubar-sub-content" className={cn(menuContent, className)} {...props} />
    </MenubarPrimitive.Portal>
  )
}

export {
  Menubar, MenubarCheckboxItem, MenubarContent, MenubarItem, MenubarLabel, MenubarMenu, MenubarSeparator,
  MenubarShortcut, MenubarSub, MenubarSubContent, MenubarSubTrigger, MenubarTrigger,
}
