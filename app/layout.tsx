import type {Metadata} from 'next';
import './globals.css';
export const metadata:Metadata = {title:'Hasjrat OneCRM · Assessment Demo',description:'Concept assessment by Putra Irawan. Omnichannel engagement and intelligent sales. All customer data is synthetic.'};
export default function RootLayout({children}:{children:React.ReactNode}) {return <html lang="id"><body>{children}</body></html>;}
