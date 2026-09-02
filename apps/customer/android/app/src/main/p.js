const fs=require('fs');let s=fs.readFileSync('AndroidManifest.xml','utf8');
const a='<manifest xmlns:android="http://schemas.android.com/apk/res/android">\n';
const b=`<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <!-- Vendndodhja kërkohet vetëm kur klienti nis një udhëtim, dhe vetëm sa është aplikacioni hapur.
         Asnjë leje sfondi: aplikacioni i klientit nuk ndjek askënd (§57). -->
    <uses-permission android:name="android.permission.ACCESS_FINE_LOCATION" />
    <uses-permission android:name="android.permission.ACCESS_COARSE_LOCATION" />
    <uses-permission android:name="android.permission.INTERNET" />
`;
if(!s.includes(a)){console.error('MISS');process.exit(1)}
s=s.replace(a,b);
fs.writeFileSync('AndroidManifest.xml',s);
console.log('ok');
