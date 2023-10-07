import van from './vanjs/van-1.2.1.debug.js'

const html = van.tags

// type Yo_apiReflect = {
//     Methods: Yo_apiReflectMethod[]
//     Types: { [_: string]: { [_: string]: string } }
// }

// type Yo_apiReflectMethod = {
//     In: string
//     Out: string
//     Path: string
// }

export function onInit(apiRefl: any) {
    van.add(document.body, html.div({}, html.pre(JSON.stringify(apiRefl))))
}
