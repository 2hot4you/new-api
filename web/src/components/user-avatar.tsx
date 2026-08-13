/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMemo } from 'react'

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import {
  getDiceBearDylanAvatarUrl,
  getUserAvatarFallback,
  getUserAvatarStyle,
} from '@/lib/avatar'
import { cn } from '@/lib/utils'

interface UserAvatarProps {
  userId: number
  name: string
  className?: string
  imageClassName?: string
  fallbackClassName?: string
}

export function UserAvatar(props: UserAvatarProps) {
  const avatarUrl = useMemo(
    () => getDiceBearDylanAvatarUrl(props.userId),
    [props.userId]
  )
  const fallbackStyle = useMemo(
    () => getUserAvatarStyle(props.name),
    [props.name]
  )

  return (
    <Avatar className={props.className}>
      <AvatarImage
        src={avatarUrl}
        alt={`@${props.name}`}
        className={props.imageClassName}
      />
      <AvatarFallback
        className={cn('font-semibold text-white', props.fallbackClassName)}
        style={fallbackStyle}
      >
        {getUserAvatarFallback(props.name)}
      </AvatarFallback>
    </Avatar>
  )
}
