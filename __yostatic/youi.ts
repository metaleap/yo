import van, { State } from './vanjs/van-1.2.6.js'
import * as youtil from './util.js'


export type DomLive<T extends { [_: string]: any }> = {
    domNode: Element
    all: T[]
    itemCount: State<number>
    timeLastModifiedDomWise: State<number>
    replaceWith: (_: T[]) => void
}

export function domLive<T extends { [_: string]: any }>(domNode: Element, initial: T[], perItem: (_: T) => Element, identPropName = 'Id'): DomLive<T> {
    type _DomLive = DomLive<T> & {
        _lastNodes: { [_: string | number]: Element }
        _lastItems: { [_: string | number]: T }
    }
    const me: _DomLive = {
        _lastNodes: {} as { [_: string | number]: Element },
        _lastItems: {} as { [_: string | number]: T },

        all: initial,
        itemCount: van.state(initial.length),
        timeLastModifiedDomWise: van.state(0),
        domNode: domNode,
        replaceWith: (items: T[]) => {
            let dom_muts = false
            // find dom nodes to remove, then remove them
            const del_nodes: Element[] = []
            for (const id in me._lastItems) {
                if (!items.some(_ => (id == (_[identPropName]!)))) { // no `===` here due to string-vs-number ambiguity
                    const node_old = me._lastNodes[id]
                    del_nodes.push(node_old)
                    delete me._lastNodes[id]
                    delete me._lastItems[id]
                }
            }
            if (dom_muts = (del_nodes.length > 0))
                for (const del_node of del_nodes)
                    del_node.replaceWith()
            // ignoring changes in sort order for now here, actual node-(re)create ops per item (if changed)
            const new_nodes = [] as Element[]
            for (let i = 0, l = items.length; i < l; i++) {
                const item = items[i]
                const item_id = item[identPropName]!
                const node_old = me._lastNodes[item_id], item_old = me._lastItems[item_id]
                if (!youtil.deepEq(item, item_old, false, false)) {
                    const node_new = perItem(item)
                    if (!node_old) // new dom append
                        new_nodes.push(node_new)
                    else  // change dom node
                        node_old.replaceWith(node_new)
                    me._lastNodes[item_id] = node_new
                }
                me._lastItems[item_id] = item
            }
            if (new_nodes.length > 0) {
                dom_muts = true
                me.domNode.append(...new_nodes)
            }
            // ensure up-to-date sort order
            for (let i = 0, l = (items.length - 1); i < l; i++) {
                const node_this = me._lastNodes[items[i][identPropName]!], node_next = me._lastNodes[items[i + 1][identPropName]!]
                if (node_this.nextElementSibling !== node_next) {
                    dom_muts = true
                    me.domNode.insertBefore(node_this, node_next)
                }
            }

            me.all = items
            me.itemCount.val = items.length
            if (dom_muts)
                me.timeLastModifiedDomWise.val = Date.now()
        }
    }
    me.replaceWith(initial)
    return me
}
