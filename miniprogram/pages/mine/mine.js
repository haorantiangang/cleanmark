const app = getApp()

Page({
  data: {
    userInfo: null,
    isVip: false,
    remainingQuota: 0,
    dailyQuota: 0,
    quotaPercent: 0,
    isLoggedIn: false,
    tasks: [],
    taskCount: 0
  },

  onShow() {
    this.loadUserInfo()
  },

  loadUserInfo() {
    const that = this

    if (app.globalData.token) {
      that.setData({ 
        isLoggedIn: true,
        userInfo: app.globalData.userInfo || {},
        isVip: app.globalData.isVip || false,
        remainingQuota: app.globalData.remainingQuota || 0,
        dailyQuota: app.globalData.dailyQuota || 3
      })

      that.setData({
        quotaPercent: that.data.dailyQuota > 0 ? (that.data.remainingQuota / that.data.dailyQuota * 100) : 0
      })

      that.loadTaskList()
    } else {
      that.setData({ 
        isLoggedIn: false,
        userInfo: null
      })
    }
  },

  loadTaskList() {
    const that = this

    wx.request({
      url: `${app.globalData.baseUrl}/tasks?page=1&page_size=10`,
      method: 'GET',
      header: {
        'Authorization': `Bearer ${app.globalData.token}`
      },
      success(res) {
        if (res.data.code === 200) {
          that.setData({ 
            tasks: res.data.data.list || [],
            taskCount: res.data.data.total || 0
          })
        }
      },
      fail(err) {
        console.error('加载任务列表失败:', err)
      }
    })
  },

  handleLogin() {
    if (!this.data.isLoggedIn) {
      app.login(() => {
        this.onShow()
      })
    } else {
      app.logout()
      this.setData({ 
        isLoggedIn: false,
        userInfo: null,
        tasks: []
      })
    }
  },

  viewTaskDetail(e) {
    const taskId = e.currentTarget.dataset.id
    wx.navigateTo({
      url: `/pages/result/result?taskId=${taskId}`
    })
  },

  refreshData() {
    wx.showLoading({ title: '刷新中...' })
    
    app.getUserInfo()
    
    setTimeout(() => {
      this.onShow()
      wx.hideLoading()
      wx.showToast({ title: '已刷新', icon: 'success' })
    }, 500)
  },

  contactService() {
    wx.showModal({
      title: '联系客服',
      content: '如有问题，请添加客服微信：cleanmark_service',
      showCancel: false,
      confirmText: '知道了'
    })
  },

  aboutApp() {
    wx.showModal({
      title: '关于CleanMark',
      content: '版本：1.0.0\n\n一款强大的去水印工具\n支持8大主流平台\n\nMade with ❤️',
      showCancel: false,
      confirmText: '好的'
    })
  },

  shareApp() {
    return {
      title: 'CleanMark - 去水印神器',
      path: '/pages/index/index',
      imageUrl: ''
    }
  },

  onShareAppMessage() {
    return this.shareApp()
  }
})
