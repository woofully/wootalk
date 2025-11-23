import type { Metadata } from 'next'
import './globals.css'

export const metadata: Metadata = {
  title: 'WooTalk - Anonymous Chat',
  description: 'Chat anonymously with people nearby',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  )
}
