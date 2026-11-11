// The shapes the API sends.
import type { GameDoc, Rule } from '../engine/types'

export interface User { id: number; email: string; name: string; role: 'admin' | 'member'; created_at: string }
export interface APIToken { id: number; user_id: number; name: string; prefix: string; last_used_at: string | null; created_at: string }

export type ConnMode = 'core' | 'direct' | 'off'
export type ConnVia = 'core' | 'direct' | ''
export type AppSlug = 'photos' | 'files' | 'draw' | 'photolab' | 'music' | 'voice' | 'translate' | 'puzzlecraft' | 'blog' | 'chat' | 'forum' | 'email' | 'contacts' | 'tasks' | 'projects' | 'notes' | 'calendar' | 'family' | 'alerts'
type ConnFields = { [K in AppSlug as `${K}_mode`]: ConnMode } & { [K in AppSlug as `${K}_via`]: ConnVia } & { [K in AppSlug as `${K}_detail` | `${K}_url` | `${K}_token` | `${K}_public_url` | `${K}_open_url`]: string } & { [K in AppSlug as `${K}_configured`]: boolean }

export type Settings = ConnFields & {
  core_configured: boolean
  core_registered?: boolean
  title: string; date_format: string; time_format: string; week_start: number
  maker_name: string; scores_on: boolean; auto_save: boolean; show_grid: boolean; sound_on: boolean; default_size: string
  files_folder: string; tasks_list: string; chat_to: string; ai_help: boolean
  notify_best: boolean; notify_daily: boolean; notify_hour: number
  notify_url: string; notify_format: string; notify_configured: boolean
  ai_provider: 'auto' | 'pocket' | 'openai'; ai_pocket_model: string; openai_api_key: string; openai_model: string; openai_base_url: string
  setup_complete: boolean; base_url: string; demo: boolean
}

export type AppStateName = 'on' | 'partly' | 'off' | 'link_off' | 'missing' | 'old' | 'no_core' | 'not_set_up' | 'error'
export interface AppState { slug: AppSlug; name: string; reason: string; mode: ConnMode; via: ConnVia; state: AppStateName; detail: string; fix: string; on: string[]; open_url: string }

export interface Game {
  id: number; title: string; description: string; template: string; doc?: GameDoc; saves: number; has_cover: boolean; cover_url: string
  share_key: string; share_on: boolean; scores_on: boolean; remix_on: boolean; published_at: string | null; plays: number
  favorite: boolean; archived: boolean; source: string; played_at: string | null; created_at: string; updated_at: string
  scenes: number; objects: number; rules: number; best: number; my_plays: number; todo: number
  url: string; share_url: string; embed_url?: string; changed_since_share: boolean
}

export interface Asset {
  id: number; game_id: number | null; kind: 'image' | 'sound' | 'music'; name: string; mime: string; size: number; width: number; height: number
  frames: number; seconds: number; pixels?: string; origin: string; origin_ref: string; source: string; created_at: string; url: string
}

export interface Play { id: number; game_id: number; who: string; mine: boolean; score: number; seconds: number; result: 'won' | 'lost' | 'quit'; level: string; day: string; created_at: string; game?: string }
export interface Todo { id: number; game_id: number; text: string; done: boolean; task_url: string; source: string; created_at: string }
export interface Version { id: number; game_id: number; note: string; created_at: string; doc?: GameDoc }
export interface Template { id: string; name: string; about: string; emoji: string; size: string }

export interface Home { games: number; shared: number; favorites: number; todo: number; assets: number; plays_week: number; recent: { id: number; title: string; shared: boolean; template: string }[] }

export type { GameDoc, Rule }
