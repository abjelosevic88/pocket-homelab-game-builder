import { Modal } from './ui'

const GROUPS: { title: string; keys: [string[], string][] }[] = [
  { title: 'Go to', keys: [[['g'], 'My games'], [['l'], 'Pictures & sounds'], [['p'], 'Plays'], [[','], 'Settings']] },
  { title: 'Make', keys: [[['n'], 'Make a game']] },
  { title: 'In the editor', keys: [[['1'], 'Place'], [['2'], 'Move and pick'], [['3'], 'Rub out'], [['space', 'drag'], 'Move around the level'], [['scroll'], 'Zoom'], [['⌘', 'Z'], 'Undo'], [['⌘', '⇧', 'Z'], 'Redo'], [['del'], 'Remove what is picked'], [['P'], 'Play']] },
  { title: 'Playing', keys: [[['←', '→'], 'Move'], [['↑', 'space'], 'Jump'], [['space', 'X'], 'Shoot / action']] },
  { title: 'Find', keys: [[['/'], 'Search & commands'], [['⌘', 'K'], 'Search & commands']] },
  { title: 'Window', keys: [[['['], 'Show or hide the sidebar'], [['esc'], 'Close']] },
  { title: 'Help', keys: [[['?'], 'This sheet']] },
]

export function ShortcutsSheet({ onClose }: { onClose: () => void }) {
  return (
    <Modal title="Keyboard shortcuts" onClose={onClose} size="lg">
      <div className="shortcuts">
        {GROUPS.map(g => (
          <section key={g.title}>
            <h4 className="sc-h">{g.title}</h4>
            {g.keys.map(([keys, label]) => (
              <div key={label + keys.join('')} className="sc-row"><span>{label}</span><span className="sc-keys">{keys.map(k => <kbd key={k}>{k}</kbd>)}</span></div>
            ))}
          </section>
        ))}
      </div>
    </Modal>
  )
}
