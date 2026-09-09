import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getLoginUser } from '@/api/authController'
import { DEFAULT_USERNAME } from '@/constants/user'

/**
 * 登录用户信息
 */
export const useLoginUserStore = defineStore('loginUser', () => {
  const loginUser = ref<API.User>({
    userName: DEFAULT_USERNAME,
  })

  async function fetchLoginUser() {
    try {
      const res = await getLoginUser()
      if (res.data.code === 0 && res.data.data) {
        loginUser.value = res.data.data
        return
      }
    } catch {
      // 未登录等
    }
    loginUser.value = { userName: DEFAULT_USERNAME }
  }

  function setLoginUser(newLoginUser: API.User) {
    loginUser.value = newLoginUser
  }

  return { loginUser, fetchLoginUser, setLoginUser }
})
