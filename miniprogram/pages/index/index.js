const app = getApp()

Page({
  data: {
    videoUrl: '',
    platform: '',
    platformName: '',
    showPlatformBadge: false,
    platforms: [
      { name: '抖音', icon: '📱', key: 'douyin' },
      { name: '快手', icon: '⚡', key: 'kuaishou' },
      { name: '小红书', icon: '📕', key: 'xiaohongshu' },
      { name: 'B站', icon: '📺', key: 'bilibili' },
      { name: '微博', icon: '📝', key: 'weibo' },
      { name: '西瓜', icon: '🍉', key: 'xigua' },
      { name: 'YouTube', icon: '▶️', key: 'youtube' },
      { name: 'TikTok', icon: '🎵', key: 'tiktok' }
    ],
    isParsing: false
  },

  onShow() {
    this.checkLogin()
  },

  checkLogin() {
    if (!app.globalData.token) {
      wx.showModal({
        title: '提示',
        content: '请先登录后再使用',
        confirmText: '去登录',
        success(res) {
          if (res.confirm) {
            app.login(() => {
              console.log('登录成功')
            })
          }
        }
      })
    }
  },

  onUrlInput(e) {
    const url = e.detail.value
    this.setData({ videoUrl: url })
    
    if (url.trim()) {
      this.detectPlatform(url)
    } else {
      this.setData({
        platform: '',
        platformName: '',
        showPlatformBadge: false
      })
    }
  },

  detectPlatform(url) {
    const that = this
    wx.request({
      url: `${app.globalData.baseUrl}/detect/platform`,
      method: 'POST',
      data: { url },
      success(res) {
        if (res.data.code === 200 && res.data.data.supported) {
          that.setData({
            platform: res.data.data.platform,
            platformName: res.data.data.platform_name,
            showPlatformBadge: true
          })
        } else {
          that.setData({
            platform: '',
            platformName: '',
            showPlatformBadge: false
          })
        }
      },
      fail() {
        console.error('平台检测失败')
      }
    })
  },

  selectPlatform(e) {
    const key = e.currentTarget.dataset.key
    const examples = {
      douyin: 'https://www.douyin.com/video/xxxxxxxx',
      kuaishou: 'https://v.kuaishou.com/xxxxxx',
      xiaohongshu: 'https://www.xiaohongshu.com/explore/xxxxx',
      bilibili: 'https://www.bilibili.com/video/BVxxxxxxxx',
      weibo: 'https://weibo.com/xxx',
      xigua: 'https://www.ixigua.com/xxx',
      youtube: 'https://www.youtube.com/watch?v=xxxxxxxx',
      tiktok: 'https://www.tiktok.com/@user/video/xxx'
    }

    this.setData({ videoUrl: examples[key] || '' })
    this.detectPlatform(examples[key] || '')
  },

  parseVideo() {
    const that = this
    const { videoUrl } = this.data

    if (!videoUrl.trim()) {
      wx.showToast({ title: '请输入视频链接', icon: 'none' })
      return
    }

    if (!app.globalData.token) {
      wx.showToast({ title: '请先登录', icon: 'none' })
      return
    }

    this.setData({ isParsing: true })

    wx.request({
      url: `${app.globalData.baseUrl}/tasks`,
      method: 'POST',
      header: {
        'Authorization': `Bearer ${app.globalData.token}`,
        'Content-Type': 'application/json'
      },
      data: { url: videoUrl },
      success(res) {
        if (res.data.code === 200) {
          wx.showToast({ title: '任务已提交', icon: 'success' })
          
          that.setData({ 
            isParsing: false,
            videoUrl: '',
            showPlatformBadge: false
          })

          setTimeout(() => {
            wx.navigateTo({
              url: `/pages/result/result?taskId=${res.data.data.task_id}`
            })
          }, 500)

        } else {
          wx.showToast({ title: res.data.message || '解析失败', icon: 'none' })
          that.setData({ isParsing: false })
        }
      },
      fail(err) {
        console.error('解析请求失败:', err)
        wx.showToast({ title: '网络错误', icon: 'none' })
        that.setData({ isParsing: false })
      }
    })
  },

  pasteFromClipboard() {
    const that = this
    wx.getClipboardData({
      success(res) {
        if (res.data) {
          that.setData({ videoUrl: res.data })
          that.detectPlatform(res.data)
          wx.showToast({ title: '已粘贴', icon: 'success' })
        } else {
          wx.showToast({ title: '剪贴板为空', icon: 'none' })
        }
      },
      fail(err) {
        console.error('读取剪贴板失败:', err)
        wx.showToast({ title: '读取失败', icon: 'none' })
      }
    })
  },

  scanQRCode() {
    const that = this
    wx.scanCode({
      onlyFromCamera: false,
      scanType: ['qrCode'],
      success(res) {
        that.setData({ videoUrl: res.result })
        that.detectPlatform(res.result)
        wx.showToast({ title: '识别成功', icon: 'success' })
      },
      fail(err) {
        console.error('扫码失败:', err)
        if (err.errMsg !== 'scanCode:fail cancel') {
          wx.showToast({ title: '扫码失败', icon: 'none' })
        }
      }
    })
  }
})
