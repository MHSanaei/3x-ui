const express = require('express');
const cors = require('cors');

const app = express();
const PORT = 3000;

// อนุญาตให้หน้าเว็บอื่นดึงข้อมูลได้ และให้รองรับการรับ-ส่งข้อมูลแบบ JSON
app.use(cors());
app.use(express.json());

// ------------------------------------------------
// สร้าง API Endpoint (ช่องทางรับออเดอร์)
// ------------------------------------------------

// 1. API ดึงข้อมูลบัญชี (สมมติว่าเป็นบัญชี vpn ของลูกค้า)
app.get('/api/account', (req, res) => {
    // ข้อมูลที่เราจะส่งกลับไปเป็น JSON
    const data = {
        status: "success",
        accountName: "2zong2",
        protocol: "VLESS",
        expireDays: 999,
        usedGB: 21.39
    };
    
    // สั่งให้ส่ง data กลับไป
    res.json(data);
});

// 2. API รับคำสั่ง (เช่น กดปุ่มรีเซ็ตจากหน้าเว็บ)
app.post('/api/action', (req, res) => {
    // อ่านข้อมูล JSON ที่หน้าเว็บส่งมา
    const userAction = req.body.action; 
    
    console.log("ได้รับคำสั่งจากหน้าเว็บ:", userAction);

    res.json({
        status: "success",
        message: `ทำคำสั่ง ${userAction} สำเร็จแล้ว!`
    });
});

// ------------------------------------------------
// สั่งให้ API เริ่มทำงาน (เปิดร้านรับลูกค้า)
// ------------------------------------------------
app.listen(PORT, () => {
    console.log(`🚀 API Server รันอยู่บน http://localhost:${PORT}`);
});
