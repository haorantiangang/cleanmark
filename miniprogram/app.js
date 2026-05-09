App({
  globalData: {
    userInfo: null,
    token: null,
    baseUrl: 'http://localhost:8080/api/v1',
    isVip: false
  },

  onLaunch() {
    this.checkLoginStatus()
  },

  checkLoginStatus() {
    const token = wx.getStorageSync('token')
    if (token) {
      this.globalData.token = token
      this.getUserInfo()
    }
  },

  getUserInfo() {
    const that = this
    wx.request({
      url: `${this.globalData.baseUrl}/user/info`,
      method: 'GET',
      header: {
        'Authorization': `Bearer ${this.globalData.token}`
      },
      success(res) {
        if (res.data.code === 200) {
          that.globalData.userInfo = res.data.data.user
          that.globalData.isVip = res.data.data.user.vip_level > 0
          that.globalData.remainingQuota = res.data.data.remaining_quota
          that.globalData.dailyQuota = res.data.data.daily_quota
        }
      },
      fail(err) {
        console.error('获取用户信息失败:', err)
      }
    })
  },

  login(successCallback) {
    const that = this
    
    wx.login({
      success(res) {
        if (res.code) {
          wx.request({
            url: `${that.globalData.baseUrl}/auth/wechat/login`,
            method: 'POST',
            data: {
              code: res.code,
              nickname: '微信用户'
            },
            success(loginRes) {
              if (loginRes.data.code === 200) {
                const data = loginRes.data.data
                
                that.globalData.token = data.token
                that.globalData.userInfo = data.user
                that.globalData.isVip = data.user.vip_level > 0
                
                wx.setStorageSync('token', data.token)
                
                wx.showToast({
                  title: '登录成功',
                  icon: 'success'
                })

                if (successCallback && typeof successCallback === 'function') {
                  successCallback(data)
                }
              } else {
                wx.showToast({
                  title: loginRes.data.message || '登录失败',
                  icon: 'none'
                })
              }
            },
            fail(err) {
              console.error('登录请求失败:', err)
              wx.showToast({
                title: '网络错误',
                icon: 'none'
              })
            }
          })
        } else {
          wx.showToast({
            title: '获取code失败',
            icon: 'none'
          })
        }
      },
      fail(err) {
        console.error('wx.login失败:', err)
        wx.showToast({
          title: '微信登录失败',
          icon: 'none'
        })
      }
    })
  },

  logout() {
    this.globalData.token = null
    this.globalData.userInfo = null
    this.globalData.isVip = false
    
    wx.removeStorageSync('token')
    
    wx.showToast({
      title: '已退出登录',
      icon: 'success'
    })
  }
})
