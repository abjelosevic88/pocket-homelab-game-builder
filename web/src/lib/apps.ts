// The other Pocket apps Game Builder works with.
import type { AppSlug, AppStateName } from './types'

export const APP_INFO: Record<AppSlug, { name: string; icon: string }> = {
  photos: { name: 'Pocket Photos', icon: 'image' }, files: { name: 'Pocket Files', icon: 'archive' }, draw: { name: 'Pocket Draw', icon: 'pen' },
  photolab: { name: 'Pocket Photo Lab', icon: 'sliders' }, music: { name: 'Pocket Music', icon: 'music' }, voice: { name: 'Pocket Voice', icon: 'mic' },
  translate: { name: 'Pocket Translate', icon: 'globe' }, puzzlecraft: { name: 'Pocket Puzzlecraft', icon: 'grid' }, blog: { name: 'Pocket Blog', icon: 'file-text' },
  chat: { name: 'Pocket Chat', icon: 'message' }, forum: { name: 'Pocket Forum', icon: 'messages' }, email: { name: 'Pocket Email', icon: 'mail' },
  contacts: { name: 'Pocket Contacts', icon: 'users' }, tasks: { name: 'Pocket Tasks', icon: 'check-square' }, projects: { name: 'Pocket Projects', icon: 'layers' },
  notes: { name: 'Pocket Notes', icon: 'note' }, calendar: { name: 'Pocket Calendar', icon: 'calendar' }, family: { name: 'Pocket Family', icon: 'home' },
  alerts: { name: 'Pocket Alerts', icon: 'bell' },
}

export const STATE_WORDS: Record<AppStateName, string> = {
  on: 'Connected', partly: 'Partly connected', off: 'Off', link_off: 'Link is off in Pocket Core', missing: 'Not on this box', old: 'Needs an update',
  no_core: 'No Pocket Core', not_set_up: 'Not set up', error: 'Not working',
}
