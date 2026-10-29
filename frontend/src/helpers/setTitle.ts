import {pageTitle} from '@/johnnys-brain/brand'

export function setTitle(title : undefined | string) {
	document.title = pageTitle(title)
}
