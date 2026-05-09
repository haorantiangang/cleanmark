const app = getApp()

Page({
  data: {
    taskId: null,
    task: null,
    status: '',
    polling: false,
    pollCount: 0
  },

  onLoad(options) {
    if (options.taskId) {
      this.setData({ taskId: options.taskId })
      this.pollTaskResult()
    }
  },

  pollTaskResult() {
    const that = this
    const { taskId } = this.data

    if (that.data.pollCount > 30) {
      wx.showToast({ title: '解析超时', icon: 'none' })
      return
    }

    that.setData({ 
      polling: true, 
      status: 'processing',
      pollCount: that.data.pollCount + 1
    })

    wx.request({
      url: `${app.globalData.baseUrl}/tasks/${taskId}`,
      method: 'GET',
      header: {
        'Authorization': `Bearer ${app.globalData.token}`
      },
      success(res) {
        if (res.data.code === 200) {
          const task = res.data.data
          
          that.setData({ task })

          if (task.status === 'success') {
            that.setData({ 
              status: 'success',
              polling: false
            })
            wx.showToast({ title: '✅ 解析成功', icon: 'success' })
          } else if (task.status === 'failed') {
            that.setData({ 
              status: 'failed',
              polling: false
            })
            wx.showToast({ title: `❌ ${task.error_message || '解析失败'}`, icon: 'none' })
          } else {
            setTimeout(() => {
              that.pollTaskResult()
            }, 1000)
          }
        } else {
          that.setData({ polling: false })
          wx.showToast({ title: res.data.message || '获取任务失败', icon: 'none' })
        }
      },
      fail(err) {
        console.error('轮询失败:', err)
        that.setData({ polling: false })
        wx.showToast({ title: '网络错误', icon: 'none' })
      }
    })
  },

  downloadVideo() {
    const { task } = this.data
    
    if (!task || !task.clean_url) {
      wx.showToast({ title: '下载链接不可用', icon: 'none' })
      return
    }

    wx.showLoading({ title: '准备下载...' })

    wx.downloadFile({
      url: `${app.globalData.baseUrl}/download/${task.id}`,
      success(res) {
        if (res.statusCode === 200) {
          wx.saveVideoToPhotosAlbum({
            filePath: res.tempFilePath,
            success() {
              wx.hideLoading()
              wx.showToast({ title: '已保存到相册', icon: 'success' })
            },
            fail(err) {
              console.error('保存失败:', err)
              wx.hideLoading()
              
              if (err.errMsg.includes('auth deny')) {
                wx.showModal({
                  title: '提示',
                  content: '需要您授权保存到相册',
                  confirmText: '去设置',
                  success(modalRes) {
                    if (modalRes.confirm) {
                      wx.openSetting({})
                    }
                  }
                })
              } else {
                wx.showToast({ title: '保存失败', icon: 'none' })
              }
            }
          })
        } else {
          wx.hideLoading()
          wx.showToast({ title: '下载失败', icon: 'none' })
        }
      },
      fail(err) {
        console.error('下载文件失败:', err)
        wx.hideLoading()
        wx.showToast({ title: '下载失败，请重试', icon: 'none' })
      }
    })
  },

  copyLink() {
    const { task } = this.data
    
    if (!task || !task.clean_url) return

    wx.setClipboardData({
      data: task.clean_url,
      success() {
        wx.showToast({ title: '链接已复制', icon: 'success' })
      }
    })
  },

  shareToFriend() {
    const { task } = this.data
    
    if (!task) return

    return {
      title: task.title || '去水印视频',
      path: `/pages/index/index`,
      imageUrl: task.cover_url || ''
    }
  },

  onShareAppMessage() {
    return this.shareToFriend()
  }
})
