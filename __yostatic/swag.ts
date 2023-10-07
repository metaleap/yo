import van from './vanjs/van-1.2.1.debug.js'

const html = van.tags

export function onInit() {
    van.add(document.body, html.div({}, html.b(html.i("SWAG")), "gish"))
}
